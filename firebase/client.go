package firebase

import (
	"context"
	"os"
	"strconv"
	"strings"

	firebase "firebase.google.com/go/v4"
	firebaseAuth "firebase.google.com/go/v4/auth"
	"google.golang.org/api/option"
)

// ----------------------------------------------------------------------------- Contracts

type (
	// Client interface (similar to TradingClient pattern)
	Client interface {
		Auth() AuthAPI
		AuthClient() *firebaseAuth.Client
		UserManagement() UserManagement
	}

	AuthAPI interface {
		VerifyIDToken(ctx context.Context, idToken string) (TokenClaims, error)
	}

	TokenClaims interface {
		ID() string
		Email() string
		EmailVerified() bool
		Claims() map[string]interface{}
	}
)

const (
	//nolint: gosec // credentials are not here, it's just the envvar name
	appCredsEnvarName = "GOOGLE_APPLICATION_CREDENTIALS"
)

// ----------------------------------------------------------------------------- Client Implementation

type (
	client struct {
		authCli *firebaseAuth.Client

		auth AuthAPI
	}

	clientOption func(c *clientConfig)

	clientConfig struct {
		firebaseOpts []option.ClientOption
	}

	authService struct {
		auth *firebaseAuth.Client
	}

	firebaseTokenDTO struct {
		id            string
		email         string
		emailVerified bool
		claims        map[string]interface{}
	}
)

func WithClientFirebaseOpts(opts ...option.ClientOption) clientOption {
	return func(c *clientConfig) {
		c.firebaseOpts = opts
	}
}

func jsonCredsFromEnv() option.ClientOption {
	// First try the specific JSON content env var
	if jsonContent := os.Getenv("FIREBASE_CREDENTIALS_JSON"); jsonContent != "" {
		return option.WithCredentialsJSON([]byte(jsonContent))
	}

	// Fallback to standard GOOGLE_APPLICATION_CREDENTIALS
	appCreds := os.Getenv(appCredsEnvarName)
	if appCreds == "" {
		// No credentials found, return empty option (let default chain handle it or fail later)
		return option.WithCredentialsFile("")
	}

	// If it looks like a path (starts with / or .), treat as file
	if strings.HasPrefix(appCreds, "/") || strings.HasPrefix(appCreds, ".") {
		return option.WithCredentialsFile(appCreds)
	}

	// Otherwise try to treat it as JSON content (legacy/fallback)
	if strings.HasPrefix(appCreds, "\"") {
		creds, err := strconv.Unquote(appCreds)
		if err != nil {
			panic(err)
		}
		appCreds = creds
	}

	return option.WithCredentialsJSON([]byte(appCreds))
}

func defaultClientOpts() []clientOption {
	return []clientOption{WithClientFirebaseOpts(jsonCredsFromEnv())}
}

func NewClient(opts ...clientOption) *client {
	c := new(clientConfig)
	for _, opt := range append(defaultClientOpts(), opts...) {
		opt(c)
	}
	appContext := context.Background()
	app, err := firebase.NewApp(appContext, nil, c.firebaseOpts...)
	if err != nil {
		panic(err)
	}

	authCli, err := app.Auth(appContext)
	if err != nil {
		panic(err)
	}

	return &client{
		authCli: authCli,
		auth:    &authService{auth: authCli},
	}
}

func (c *client) Auth() AuthAPI {
	return c.auth
}

func (c *client) AuthClient() *firebaseAuth.Client {
	return c.authCli
}

// ----------------------------------------------------------------------------- Auth Implementation

func (s *authService) VerifyIDToken(ctx context.Context, idToken string) (TokenClaims, error) {
	token, err := s.auth.VerifyIDToken(ctx, idToken)
	if err != nil {
		return nil, err
	}

	email, _ := token.Claims["email"].(string)
	emailVerified, _ := token.Claims["email_verified"].(bool)

	return &firebaseTokenDTO{
		id:            token.UID,
		email:         email,
		emailVerified: emailVerified,
		claims:        token.Claims,
	}, nil
}

// ----------------------------------------------------------------------------- User Management Implementation

func (c *client) UserManagement() UserManagement {
	return &userManagement{auth: c.authCli}
}

type userManagement struct {
	auth *firebaseAuth.Client
}

func (u *userManagement) Create(ctx context.Context, user User) (User, error) {
	params := (&firebaseAuth.UserToCreate{}).
		Email(user.Email()).
		Password(user.Password()).
		DisplayName(user.FirstName() + " " + user.LastName()).
		EmailVerified(user.EmailVerified()).
		Disabled(false)

	if user.ID() != "" {
		params = params.UID(user.ID())
	}

	record, err := u.auth.CreateUser(ctx, params)
	if err != nil {
		return nil, err
	}

	return u.mapRecordToUser(record), nil
}

func (u *userManagement) Get(ctx context.Context, id string) (User, error) {
	record, err := u.auth.GetUser(ctx, id)
	if err != nil {
		return nil, err
	}
	return u.mapRecordToUser(record), nil
}

func (u *userManagement) GetByEmail(ctx context.Context, email string) (User, error) {
	record, err := u.auth.GetUserByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	return u.mapRecordToUser(record), nil
}

func (u *userManagement) Update(ctx context.Context, id string, user User) (User, error) {
	params := (&firebaseAuth.UserToUpdate{}).
		Email(user.Email()).
		DisplayName(user.FirstName() + " " + user.LastName()).
		EmailVerified(user.EmailVerified())

	if user.Password() != "" {
		params = params.Password(user.Password())
	}

	record, err := u.auth.UpdateUser(ctx, id, params)
	if err != nil {
		return nil, err
	}

	return u.mapRecordToUser(record), nil
}

func (u *userManagement) Delete(ctx context.Context, id string) error {
	return u.auth.DeleteUser(ctx, id)
}

func (u *userManagement) mapRecordToUser(r *firebaseAuth.UserRecord) User {
	nameParts := strings.SplitN(r.DisplayName, " ", 2)
	firstName := r.DisplayName
	lastName := ""
	if len(nameParts) > 1 {
		firstName = nameParts[0]
		lastName = nameParts[1]
	}

	return &user{
		id:            r.UID,
		email:         r.Email,
		emailVerified: r.EmailVerified,
		firstName:     firstName,
		lastName:      lastName,
	}
}

// ----------------------------------------------------------------------------- Interface Implementations

// TokenClaims interface implementation
func (t *firebaseTokenDTO) ID() string                     { return t.id }
func (t *firebaseTokenDTO) Email() string                  { return t.email }
func (t *firebaseTokenDTO) EmailVerified() bool            { return t.emailVerified }
func (t *firebaseTokenDTO) Claims() map[string]interface{} { return t.claims }
