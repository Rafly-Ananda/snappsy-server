package services

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/rafly-ananda/snappsy-uploader-api/internal/config"
	"github.com/rafly-ananda/snappsy-uploader-api/internal/models"
	"github.com/rafly-ananda/snappsy-uploader-api/internal/repositories"
	"github.com/rafly-ananda/snappsy-uploader-api/internal/storage"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"
)

type DriveEnqueuer interface {
	Enqueue(ctx context.Context, item models.MediaItem) error
}

type DriveCleaner interface {
	DeleteMediaFiles(ctx context.Context, eventID, mediaID string) error
}

var (
	ErrDriveSetupTokenInvalid = errors.New("drive setup token is invalid or expired")
	ErrDriveQueueFull         = errors.New("drive export queue is full")
)

type DriveService struct {
	cfg    config.DriveConfig
	repo   repositories.DriveRepository
	events repositories.EventRepository
	media  repositories.MediaRepository
	obj    storage.ObjectStorage
	bucket string
	oauth  *oauth2.Config
	cipher *tokenCipher
}

func NewDriveService(cfg config.DriveConfig, repo repositories.DriveRepository, events repositories.EventRepository, media repositories.MediaRepository, obj storage.ObjectStorage, bucket string) *DriveService {
	svc := &DriveService{cfg: cfg, repo: repo, events: events, media: media, obj: obj, bucket: bucket}
	if !cfg.Configured() {
		return svc
	}
	svc.oauth = &oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		RedirectURL:  cfg.RedirectURL,
		Scopes:       []string{drive.DriveScope, "openid", "email", "https://www.googleapis.com/auth/userinfo.email"},
		Endpoint:     google.Endpoint,
	}
	if cipher, err := newTokenCipher(cfg.TokenKey); err == nil {
		svc.cipher = cipher
	}
	return svc
}

func (s *DriveService) Configured() bool {
	return s != nil && s.oauth != nil && s.cipher != nil
}

func (s *DriveService) OperatorSession(ctx context.Context) (string, string, string, error) {
	conn, err := s.repo.FindOperatorConnection(ctx)
	if err != nil {
		return "", "", "", ErrDriveNotConfigured
	}
	token, err := s.accessToken(ctx, conn.ID.Hex())
	if err != nil {
		return "", "", "", err
	}
	return token, s.cfg.PickerAPIKey, s.cfg.PickerAppID, nil
}

func (s *DriveService) PickerConfig() (string, string) {
	return s.cfg.PickerAPIKey, s.cfg.PickerAppID
}

func (s *DriveService) OAuthStart(ctx context.Context, kind, customerID, eventID, setupToken string) (string, error) {
	if !s.Configured() {
		return "", ErrDriveNotConfigured
	}
	rawState, err := randomDriveToken()
	if err != nil {
		return "", err
	}
	state := models.DriveOAuthState{
		StateHash:  hashToken(rawState),
		Kind:       kind,
		CustomerID: customerID,
		EventID:    eventID,
		ExpiresAt:  time.Now().Add(10 * time.Minute),
		CreatedAt:  time.Now(),
	}
	if setupToken != "" {
		state.SetupTokenHash = hashToken(setupToken)
		state.SetupTokenEnc, err = s.cipher.Encrypt(setupToken)
		if err != nil {
			return "", err
		}
	}
	if err := s.repo.CreateOAuthState(ctx, state); err != nil {
		return "", err
	}
	return s.oauth.AuthCodeURL(rawState, oauth2.AccessTypeOffline, oauth2.ApprovalForce), nil
}

func (s *DriveService) OAuthCallback(ctx context.Context, rawState, code string) (string, error) {
	if !s.Configured() {
		return "", ErrDriveNotConfigured
	}
	state, err := s.repo.ConsumeOAuthState(ctx, hashToken(rawState), time.Now())
	if err != nil {
		return "", ErrDriveSetupTokenInvalid
	}
	token, err := s.oauth.Exchange(ctx, code)
	if err != nil {
		return "", fmt.Errorf("google oauth exchange: %w", err)
	}
	if token.RefreshToken == "" {
		return "", errors.New("google did not return a refresh token; reconnect with consent")
	}
	encrypted, err := s.cipher.Encrypt(token.RefreshToken)
	if err != nil {
		return "", err
	}
	conn := models.DriveConnection{
		Kind:                  models.DriveConnectionKind(state.Kind),
		CustomerID:            state.CustomerID,
		Email:                 s.fetchEmail(ctx, token.AccessToken),
		EncryptedRefreshToken: encrypted,
		Scope:                 strings.Join(s.oauth.Scopes, " "),
		Status:                models.DriveConnectionActive,
	}
	connectionID, err := s.repo.UpsertConnection(ctx, conn)
	if err != nil {
		return "", err
	}
	if state.Kind == string(models.DriveConnectionCustomer) {
		if state.SetupTokenHash != "" {
			if err := s.repo.SetSetupConnection(ctx, state.SetupTokenHash, connectionID); err != nil {
				return "", err
			}
			rawSetupToken, err := s.cipher.Decrypt(state.SetupTokenEnc)
			if err != nil {
				return "", err
			}
			return strings.TrimRight(s.cfg.FrontendURL, "/") + "/drive/setup/" + rawSetupToken + "?connected=1", nil
		}
		if state.CustomerID != "" {
			redirect := strings.TrimRight(s.cfg.FrontendURL, "/") + "/dashboard"
			if state.EventID != "" {
				redirect += "/events/" + url.PathEscape(state.EventID)
			}
			return redirect + "?drive=connected", nil
		}
	}
	if state.EventID != "" {
		return strings.TrimRight(s.cfg.FrontendURL, "/") + "/admin/events/" + url.PathEscape(state.EventID) + "?drive=connected", nil
	}
	return strings.TrimRight(s.cfg.FrontendURL, "/") + "/admin/events?drive=connected", nil
}

func (s *DriveService) CreateSetupLink(ctx context.Context, eventID string) (string, time.Time, error) {
	if !s.Configured() {
		return "", time.Time{}, ErrDriveNotConfigured
	}
	if _, err := s.events.FindByID(ctx, eventID); err != nil {
		return "", time.Time{}, ErrEventNotFound
	}
	raw, err := randomDriveToken()
	if err != nil {
		return "", time.Time{}, err
	}
	expires := time.Now().Add(s.cfg.SetupTTL)
	if err := s.repo.CreateSetupToken(ctx, models.DriveSetupToken{
		TokenHash: hashToken(raw), EventID: eventID, ExpiresAt: expires, CreatedAt: time.Now(),
	}); err != nil {
		return "", time.Time{}, err
	}
	return strings.TrimRight(s.cfg.FrontendURL, "/") + "/drive/setup/" + raw, expires, nil
}

func (s *DriveService) SetupInfo(ctx context.Context, rawToken string) (models.Event, bool, error) {
	token, err := s.setupToken(ctx, rawToken)
	if err != nil {
		return models.Event{}, false, err
	}
	event, err := s.events.FindByID(ctx, token.EventID)
	if err != nil {
		return models.Event{}, false, ErrEventNotFound
	}
	return event, token.ConnectionID != "", nil
}

func (s *DriveService) SetupSession(ctx context.Context, rawToken string) (string, string, error) {
	token, err := s.setupToken(ctx, rawToken)
	if err != nil {
		return "", "", err
	}
	if token.ConnectionID == "" {
		return "", "", errors.New("drive authorization has not completed")
	}
	accessToken, err := s.accessToken(ctx, token.ConnectionID)
	if err != nil {
		return "", "", err
	}
	return accessToken, s.cfg.PickerAPIKey, nil
}

func (s *DriveService) SetCustomerDestination(ctx context.Context, rawToken, folderID string) error {
	token, err := s.setupToken(ctx, rawToken)
	if err != nil {
		return err
	}
	if token.ConnectionID == "" {
		return errors.New("drive authorization has not completed")
	}
	folderID = normalizeFolderID(folderID)
	folderName, err := s.validateFolder(ctx, token.ConnectionID, folderID)
	if err != nil {
		return err
	}
	eventFolderID := ""
	if existing, findErr := s.repo.FindDestinationByEvent(ctx, token.EventID); findErr == nil && existing.ConnectionID == token.ConnectionID && existing.FolderID == folderID {
		eventFolderID = existing.EventFolderID
	}
	if err := s.repo.UpsertDestination(ctx, models.DriveDestination{
		EventID: token.EventID, ConnectionID: token.ConnectionID, FolderID: folderID, FolderName: folderName, EventFolderID: eventFolderID,
	}); err != nil {
		return err
	}
	if err := s.repo.MarkSetupTokenUsed(ctx, token.TokenHash, time.Now()); err != nil {
		return err
	}
	return nil
}

func (s *DriveService) SetOperatorDestination(ctx context.Context, eventID, folderID string) (models.DriveDestination, error) {
	if _, err := s.events.FindByID(ctx, eventID); err != nil {
		return models.DriveDestination{}, ErrEventNotFound
	}
	conn, err := s.repo.FindOperatorConnection(ctx)
	if err != nil {
		return models.DriveDestination{}, ErrDriveNotConfigured
	}
	if !driveScopeGranted(conn) {
		return models.DriveDestination{}, ErrDriveReconnect
	}
	folderID = normalizeFolderID(folderID)
	folderName, err := s.validateFolder(ctx, conn.ID.Hex(), folderID)
	if err != nil {
		return models.DriveDestination{}, err
	}
	eventFolderID := ""
	if existing, findErr := s.repo.FindDestinationByEvent(ctx, eventID); findErr == nil && existing.ConnectionID == conn.ID.Hex() && existing.FolderID == folderID {
		eventFolderID = existing.EventFolderID
	}
	destination := models.DriveDestination{EventID: eventID, ConnectionID: conn.ID.Hex(), FolderID: folderID, FolderName: folderName, EventFolderID: eventFolderID}
	if err := s.repo.UpsertDestination(ctx, destination); err != nil {
		return models.DriveDestination{}, err
	}
	return destination, nil
}

func (s *DriveService) AdminStatus(ctx context.Context, eventID string) (map[string]any, error) {
	operatorConn, operatorErr := s.repo.FindOperatorConnection(ctx)
	destination, destinationErr := s.repo.FindDestinationByEvent(ctx, eventID)
	counts, _ := s.repo.ExportSummaryByEvent(ctx, eventID)
	return map[string]any{
		"configured":        s.Configured(),
		"pickerConfigured":  s.cfg.PickerConfigured(),
		"operatorConnected": operatorErr == nil && driveScopeGranted(operatorConn),
		"destination":       destinationOrDefault(destination, destinationErr),
		"counts":            counts,
	}, nil
}

func destinationOrDefault(destination models.DriveDestination, err error) any {
	if err != nil {
		return nil
	}
	return destination
}

func (s *DriveService) CustomerStatus(ctx context.Context, customerID, eventID string) (map[string]any, error) {
	if _, err := s.customerEvent(ctx, customerID, eventID); err != nil {
		return nil, err
	}
	conn, connErr := s.repo.FindCustomerConnection(ctx, customerID)
	connected := connErr == nil && driveScopeGranted(conn)
	destination, destinationErr := s.repo.FindDestinationByEvent(ctx, eventID)
	counts, _ := s.repo.ExportSummaryByEvent(ctx, eventID)
	email := ""
	if connected {
		email = conn.Email
	}
	return map[string]any{
		"configured":        s.Configured(),
		"customerConnected": connected,
		"connectionEmail":   email,
		"destination":       destinationOrDefault(destination, destinationErr),
		"counts":            counts,
	}, nil
}

func (s *DriveService) CustomerSession(ctx context.Context, customerID, eventID string) (string, string, string, error) {
	if _, err := s.customerEvent(ctx, customerID, eventID); err != nil {
		return "", "", "", err
	}
	conn, err := s.repo.FindCustomerConnection(ctx, customerID)
	if err != nil {
		return "", "", "", ErrDriveNotConfigured
	}
	if !driveScopeGranted(conn) {
		return "", "", "", ErrDriveReconnect
	}
	token, err := s.accessToken(ctx, conn.ID.Hex())
	if err != nil {
		return "", "", "", err
	}
	return token, s.cfg.PickerAPIKey, s.cfg.PickerAppID, nil
}

func (s *DriveService) StartCustomerOAuth(ctx context.Context, customerID, eventID string) (string, error) {
	if _, err := s.customerEvent(ctx, customerID, eventID); err != nil {
		return "", err
	}
	return s.OAuthStart(ctx, string(models.DriveConnectionCustomer), customerID, eventID, "")
}

func (s *DriveService) SetCustomerDestinationForOwner(ctx context.Context, customerID, eventID, folderID string) (models.DriveDestination, error) {
	if _, err := s.customerEvent(ctx, customerID, eventID); err != nil {
		return models.DriveDestination{}, err
	}
	conn, err := s.repo.FindCustomerConnection(ctx, customerID)
	if err != nil {
		return models.DriveDestination{}, ErrDriveNotConfigured
	}
	if !driveScopeGranted(conn) {
		return models.DriveDestination{}, ErrDriveReconnect
	}
	folderID = normalizeFolderID(folderID)
	folderName, err := s.validateFolder(ctx, conn.ID.Hex(), folderID)
	if err != nil {
		return models.DriveDestination{}, fmt.Errorf("%w: %v", ErrDriveFolderInaccessible, err)
	}
	eventFolderID := ""
	if existing, findErr := s.repo.FindDestinationByEvent(ctx, eventID); findErr == nil && existing.ConnectionID == conn.ID.Hex() && existing.FolderID == folderID {
		eventFolderID = existing.EventFolderID
	}
	destination := models.DriveDestination{EventID: eventID, ConnectionID: conn.ID.Hex(), FolderID: folderID, FolderName: folderName, EventFolderID: eventFolderID}
	if err := s.repo.UpsertDestination(ctx, destination); err != nil {
		return models.DriveDestination{}, err
	}
	return destination, nil
}

func (s *DriveService) customerEvent(ctx context.Context, customerID, eventID string) (models.Event, error) {
	event, err := s.events.FindByID(ctx, eventID)
	if err != nil || event.OwnerType != models.EventOwnerCustomer || event.OwnerID != customerID {
		return models.Event{}, ErrEventNotFound
	}
	return event, nil
}

func (s *DriveService) Retry(ctx context.Context, eventID string) (int64, error) {
	return s.repo.RetryFailedByEvent(ctx, eventID)
}

func (s *DriveService) Enqueue(ctx context.Context, item models.MediaItem) error {
	if !s.Configured() || item.ProcessingState != models.MediaProcessingReady {
		return nil
	}
	destination, err := s.repo.FindDestinationByEvent(ctx, item.EventID)
	if err != nil {
		return nil
	}
	counts, _ := s.repo.ExportSummaryByEvent(ctx, item.EventID)
	if counts["pending"]+counts["exporting"] >= int64(s.cfg.QueueLimit) {
		return ErrDriveQueueFull
	}
	if destination.ConnectionID == "" {
		return nil
	}
	return s.repo.InsertExportJobIfMissing(ctx, exportJobFor(item))
}

func driveGuestHash(item models.MediaItem) string {
	guestHash := item.GuestIDHash
	if guestHash == "" {
		guestHash = hashToken("legacy:" + strings.ToLower(item.Username))
	}
	return hashToken("drive:" + guestHash + ":" + strings.ToLower(strings.TrimSpace(item.Username)))
}

func exportJobFor(item models.MediaItem) models.DriveExportJob {
	return models.DriveExportJob{
		MediaID: item.ID.Hex(), EventID: item.EventID, GuestIDHash: driveGuestHash(item),
		State: models.DriveExportPending, MediaFileName: exportMediaName(item),
	}
}

func (s *DriveService) ReexportEvent(ctx context.Context, eventID string) (int64, error) {
	if !s.Configured() {
		return 0, ErrDriveNotConfigured
	}
	if _, err := s.events.FindByID(ctx, eventID); err != nil {
		return 0, ErrEventNotFound
	}
	if _, err := s.repo.FindDestinationByEvent(ctx, eventID); err != nil {
		return 0, errors.New("select a Google Drive destination first")
	}
	reset, err := s.repo.ResetExportsByEvent(ctx, eventID)
	if err != nil {
		return 0, err
	}
	items, err := s.media.FindReadyByEvent(ctx, eventID)
	if err != nil {
		return reset, err
	}
	for _, item := range items {
		job := exportJobFor(item)
		if err := s.repo.SetExportJobGuestHash(ctx, item.ID.Hex(), job.GuestIDHash); err != nil {
			return reset, err
		}
		if err := s.repo.InsertExportJobIfMissing(ctx, job); err != nil {
			return reset, err
		}
	}
	return reset, nil
}

func (s *DriveService) EnqueueEvent(ctx context.Context, eventID string) error {
	if !s.Configured() {
		return nil
	}
	items, err := s.media.FindReadyByEvent(ctx, eventID)
	if err != nil {
		return err
	}
	for _, item := range items {
		if err := s.Enqueue(ctx, item); err != nil {
			return err
		}
	}
	return nil
}

func (s *DriveService) DeleteMediaFiles(ctx context.Context, eventID, mediaID string) error {
	if !s.Configured() {
		return nil
	}
	destination, err := s.repo.FindDestinationByEvent(ctx, eventID)
	if err != nil {
		return s.repo.DeleteByMediaID(ctx, mediaID)
	}
	client, err := s.driveClient(ctx, destination.ConnectionID)
	if err != nil {
		return err
	}
	query := fmt.Sprintf("appProperties has { key='mediaId' and value='%s' } and trashed=false", mediaID)
	files, err := client.Files.List().Q(query).Fields("files(id)").PageSize(10).Do()
	if err != nil {
		return err
	}
	for _, file := range files.Files {
		if err := client.Files.Delete(file.Id).Do(); err != nil && !isNotFound(err) {
			return err
		}
	}
	return s.repo.DeleteByMediaID(ctx, mediaID)
}

func (s *DriveService) Run(ctx context.Context) {
	if !s.Configured() {
		log.Println("drive worker disabled: google drive is not configured")
		return
	}
	concurrency := s.cfg.Concurrency
	if concurrency < 1 {
		concurrency = 1
	}
	for i := 0; i < concurrency; i++ {
		go s.workerLoop(ctx)
	}
	<-ctx.Done()
}

func (s *DriveService) workerLoop(ctx context.Context) {
	for {
		if ctx.Err() != nil {
			return
		}
		job, ok, err := s.repo.ClaimExportJob(ctx, time.Now(), 5*time.Minute)
		if err != nil {
			log.Printf("drive claim failed: %v", err)
			time.Sleep(5 * time.Second)
			continue
		}
		if !ok {
			select {
			case <-ctx.Done():
				return
			case <-time.After(3 * time.Second):
			}
			continue
		}
		if err := s.exportJob(ctx, job); err != nil {
			s.failOrRetry(ctx, job, err)
		}
	}
}

func (s *DriveService) exportJob(ctx context.Context, job models.DriveExportJob) error {
	media, err := s.media.FindByID(ctx, job.MediaID)
	if err != nil {
		return err
	}
	destination, err := s.repo.FindDestinationByEvent(ctx, job.EventID)
	if err != nil {
		return fmt.Errorf("drive destination missing: %w", err)
	}
	connection, err := s.repo.FindConnection(ctx, destination.ConnectionID)
	if err != nil {
		return err
	}
	if connection.Status != models.DriveConnectionActive {
		return errors.New("drive connection is not active")
	}
	client, err := s.driveClient(ctx, destination.ConnectionID)
	if err != nil {
		return err
	}
	mediaFileID, err := s.uploadOrFind(ctx, client, destination.FolderID, exportMediaName(media), mediaContentType(media), func() (io.ReadCloser, error) {
		key := mediaExportKey(media)
		if key == "" {
			return nil, errors.New("media object key is empty")
		}
		return s.obj.Get(ctx, s.bucket, key)
	}, map[string]string{"mediaId": job.MediaID, "eventId": job.EventID, "kind": "media"})
	if err != nil {
		return err
	}
	return s.repo.CompleteExportJob(ctx, job.ID.Hex(), mediaFileID, "")
}

func (s *DriveService) failOrRetry(ctx context.Context, job models.DriveExportJob, cause error) {
	code := safeDriveError(cause)
	if job.Attempts < 3 {
		if err := s.repo.RescheduleExportJob(ctx, job.ID.Hex(), code); err != nil {
			log.Printf("drive reschedule job=%s: %v", job.ID.Hex(), err)
		}
		return
	}
	if err := s.repo.FailExportJob(ctx, job.ID.Hex(), code); err != nil {
		log.Printf("drive fail job=%s: %v", job.ID.Hex(), err)
	}
}

func (s *DriveService) driveClient(ctx context.Context, connectionID string) (*drive.Service, error) {
	conn, err := s.repo.FindConnection(ctx, connectionID)
	if err != nil {
		return nil, err
	}
	if conn.Status != models.DriveConnectionActive {
		return nil, errors.New("drive connection is not active")
	}
	refresh, err := s.cipher.Decrypt(conn.EncryptedRefreshToken)
	if err != nil {
		return nil, err
	}
	source := s.oauth.TokenSource(ctx, &oauth2.Token{RefreshToken: refresh})
	token, err := source.Token()
	if err != nil {
		return nil, err
	}
	return drive.NewService(ctx, option.WithHTTPClient(oauth2.NewClient(ctx, oauth2.StaticTokenSource(token))))
}

func (s *DriveService) fetchEmail(ctx context.Context, accessToken string) string {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://www.googleapis.com/oauth2/v2/userinfo", nil)
	if err != nil {
		return ""
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return ""
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return ""
	}
	var payload struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return ""
	}
	return payload.Email
}

func (s *DriveService) accessToken(ctx context.Context, connectionID string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	conn, err := s.repo.FindConnection(ctx, connectionID)
	if err != nil {
		return "", err
	}
	if conn.Status != models.DriveConnectionActive {
		return "", errors.New("drive connection is not active")
	}
	refresh, err := s.cipher.Decrypt(conn.EncryptedRefreshToken)
	if err != nil {
		return "", err
	}
	token, err := s.oauth.TokenSource(ctx, &oauth2.Token{RefreshToken: refresh}).Token()
	if err != nil {
		return "", err
	}
	return token.AccessToken, nil
}

func (s *DriveService) validateFolder(ctx context.Context, connectionID, folderID string) (string, error) {
	client, err := s.driveClient(ctx, connectionID)
	if err != nil {
		return "", err
	}
	folder, err := client.Files.Get(folderID).Fields("id,name,mimeType,trashed").Do()
	if err != nil {
		return "", fmt.Errorf("drive folder is not accessible: %w", err)
	}
	if folder.Trashed || folder.MimeType != "application/vnd.google-apps.folder" {
		return "", errors.New("drive destination must be a non-trashed folder")
	}
	test, err := client.Files.Create(&drive.File{
		Name: ".snappsy-write-test", MimeType: "text/plain", Parents: []string{folderID},
	}).Media(strings.NewReader("snappsy"), googleapi.ContentType("text/plain")).Do()
	if err != nil {
		return "", fmt.Errorf("drive folder is not writable: %w", err)
	}
	if err := client.Files.Delete(test.Id).Do(); err != nil && !isNotFound(err) {
		return "", fmt.Errorf("drive folder cleanup failed: %w", err)
	}
	return folder.Name, nil
}

func (s *DriveService) folderUsable(ctx context.Context, client *drive.Service, folderID string) bool {
	file, err := client.Files.Get(folderID).Fields("id,trashed").Context(ctx).Do()
	return err == nil && file != nil && !file.Trashed
}

func (s *DriveService) createFolder(ctx context.Context, client *drive.Service, parentID, name string) (string, error) {
	folder, err := client.Files.Create(&drive.File{
		Name: name, MimeType: "application/vnd.google-apps.folder", Parents: []string{parentID},
	}).Fields("id").Do()
	if err != nil {
		return "", err
	}
	return folder.Id, nil
}

func (s *DriveService) uploadOrFind(ctx context.Context, client *drive.Service, parentID, name, mimeType string, open func() (io.ReadCloser, error), properties map[string]string) (string, error) {
	mediaID := properties["mediaId"]
	query := fmt.Sprintf("appProperties has { key='mediaId' and value='%s' } and appProperties has { key='kind' and value='%s' } and trashed=false", mediaID, properties["kind"])
	files, err := client.Files.List().Q(query).Fields("files(id)").PageSize(1).Do()
	if err != nil {
		return "", err
	}
	if len(files.Files) > 0 {
		return files.Files[0].Id, nil
	}
	reader, err := open()
	if err != nil {
		return "", err
	}
	defer reader.Close()
	created, err := client.Files.Create(&drive.File{
		Name: name, MimeType: mimeType, Parents: []string{parentID}, AppProperties: properties,
	}).Media(reader, googleapi.ChunkSize(256*1024), googleapi.ContentType(mimeType)).Fields("id").Do()
	if err != nil {
		return "", err
	}
	return created.Id, nil
}

func (s *DriveService) setupToken(ctx context.Context, rawToken string) (models.DriveSetupToken, error) {
	token, err := s.repo.FindSetupToken(ctx, hashToken(rawToken))
	if err != nil || token.UsedAt != nil || time.Now().After(token.ExpiresAt) {
		return models.DriveSetupToken{}, ErrDriveSetupTokenInvalid
	}
	return token, nil
}

func randomDriveToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func eventFolderName(event models.Event) string {
	shortID := event.ID.Hex()
	if len(shortID) > 6 {
		shortID = shortID[len(shortID)-6:]
	}
	return fmt.Sprintf("%s [%s]", strings.TrimSpace(event.Name), shortID)
}

func driveScopeGranted(conn models.DriveConnection) bool {
	for _, scope := range strings.Fields(conn.Scope) {
		if scope == drive.DriveScope {
			return true
		}
	}
	return false
}

func normalizeFolderID(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return value
	}
	if strings.Contains(value, "/folders/") {
		value = value[strings.Index(value, "/folders/")+len("/folders/"):]
	} else if strings.Contains(value, "id=") {
		value = value[strings.Index(value, "id=")+len("id="):]
	}
	if cut := strings.IndexAny(value, "?&#"); cut >= 0 {
		value = value[:cut]
	}
	return strings.Trim(value, "/ ")
}

func safeGuestName(name string) string {
	name = strings.TrimSpace(name)
	name = strings.Map(func(r rune) rune {
		if r < 32 || r == '/' || r == '\\' {
			return -1
		}
		return r
	}, name)
	if name == "" {
		return "Guest"
	}
	if len([]rune(name)) > 80 {
		name = string([]rune(name)[:80])
	}
	return name
}

func mediaExportKey(item models.MediaItem) string {
	if item.Kind == models.MediaKindBoomerang {
		if item.OutputKey != "" {
			return item.OutputKey
		}
		return item.ObjectKey
	}
	if item.CompositeKey != "" {
		return item.CompositeKey
	}
	if item.ObjectKey != "" {
		return item.ObjectKey
	}
	return item.BaseKey
}

func exportMediaName(item models.MediaItem) string {
	label := strings.TrimSpace(item.Username)
	caption := strings.TrimSpace(item.Captions)
	if caption != "" {
		if label != "" {
			label += " - " + caption
		} else {
			label = caption
		}
	}
	if label == "" {
		label = "snappsy"
	}
	label = safeFilePart(label)
	shortID := item.ID.Hex()
	if len(shortID) > 8 {
		shortID = shortID[:8]
	}
	ext := "jpg"
	if item.Kind == models.MediaKindBoomerang {
		ext = "mp4"
	}
	return fmt.Sprintf("%s [%s].%s", label, shortID, ext)
}

func safeFilePart(value string) string {
	value = strings.Map(func(r rune) rune {
		switch {
		case r < 32:
			return -1
		case strings.ContainsRune(`/\:*?"<>|`, r):
			return '-'
		default:
			return r
		}
	}, value)
	value = strings.Join(strings.Fields(value), " ")
	runes := []rune(value)
	if len(runes) > 80 {
		value = strings.TrimSpace(string(runes[:80]))
	}
	if value == "" {
		return "snappsy"
	}
	return value
}

func mediaContentType(item models.MediaItem) string {
	if item.Kind == models.MediaKindBoomerang {
		return "video/mp4"
	}
	return "image/jpeg"
}

func buildMessageTXT(event models.Event, item models.MediaItem) string {
	return fmt.Sprintf("Uploader: %s\nMessage: %s\nEvent: %s\nMedia ID: %s\nSubmitted: %s\n",
		item.Username, item.Captions, event.Name, item.ID.Hex(), item.CreatedAt.UTC().Format(time.RFC3339))
}

func safeDriveError(err error) string {
	if err == nil {
		return ""
	}
	var apiErr *googleapi.Error
	if errors.As(err, &apiErr) {
		return fmt.Sprintf("drive_http_%d", apiErr.Code)
	}
	return "drive_export_failed"
}

func isNotFound(err error) bool {
	var apiErr *googleapi.Error
	return errors.As(err, &apiErr) && apiErr.Code == 404
}
