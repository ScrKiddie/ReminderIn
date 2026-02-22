package main

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"reminderin/internal/handler"
	"reminderin/internal/store"
	"reminderin/internal/whatsapp"
	"reminderin/web"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/golang-jwt/jwt/v5"
	"github.com/joho/godotenv"
)

var (
	jwtSecret         []byte
	trustProxyHeaders bool
)

const (
	maxJSONBodyBytes int64 = 1 << 20

	loginLimiterDefaultMaxStates = 10_000
	loginLimiterDefaultStateTTL  = 24 * time.Hour
	loginLimiterDefaultCleanup   = 5 * time.Minute
)

func main() {
	_ = godotenv.Load()

	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "data/reminderin.db"
	}

	dbDir := filepath.Dir(dbPath)
	if _, err := os.Stat(dbDir); os.IsNotExist(err) {
		if err := os.MkdirAll(dbDir, 0755); err != nil {
			log.Fatalf("failed to create db dir: %v", err)
		}
	}

	allowInsecureDefaults := strings.EqualFold(os.Getenv("ALLOW_INSECURE_DEFAULTS"), "true")
	trustProxyHeaders = strings.EqualFold(os.Getenv("TRUST_PROXY_HEADERS"), "true")

	adminUser := os.Getenv("REMINDERIN_USERNAME")
	adminPass := os.Getenv("REMINDERIN_PASSWORD")
	if adminUser == "" || adminPass == "" {
		if allowInsecureDefaults {
			if adminUser == "" {
				adminUser = "admin"
			}
			if adminPass == "" {
				adminPass = "admin"
			}
			log.Println("WARNING: insecure default credentials enabled via ALLOW_INSECURE_DEFAULTS=true")
		} else {
			log.Fatal("REMINDERIN_USERNAME and REMINDERIN_PASSWORD are required")
		}
	}
	if adminUser == "admin" && adminPass == "admin" && !allowInsecureDefaults {
		log.Fatal("refusing to start with default admin/admin credentials; set strong REMINDERIN_USERNAME and REMINDERIN_PASSWORD")
	}

	secretEnv := os.Getenv("JWT_SECRET")
	if secretEnv != "" {
		jwtSecret = []byte(secretEnv)
		if len(jwtSecret) < 32 {
			log.Println("WARNING: JWT_SECRET is shorter than 32 bytes; use >=32 bytes")
		}
	} else {
		if allowInsecureDefaults {
			secretBytes := make([]byte, 32)
			if _, err := rand.Read(secretBytes); err != nil {
				log.Fatalf("failed to generate random JWT secret: %v", err)
			}
			jwtSecret = secretBytes
			log.Println("WARNING: random JWT secret generated (sessions do not survive restart)")
		} else {
			log.Fatal("JWT_SECRET is required")
		}
	}

	sqliteStore, err := store.NewSQLiteStore(dbPath)
	if err != nil {
		log.Fatalf("Failed to initialize store: %v", err)
	}
	defer sqliteStore.Close()

	waMgr, err := whatsapp.NewClientManager()
	if err != nil {
		log.Fatalf("Failed to init WA manager: %v", err)
	}

	loadAllWAClients := strings.EqualFold(os.Getenv("WA_LOAD_ALL_CLIENTS"), "true")
	if loadAllWAClients {
		if err := waMgr.LoadAllClients(); err != nil {
			log.Printf("Warning: failed to load existing WA clients: %v", err)
		}
	} else {
		waNumber := sqliteStore.GetWANumber()
		if waNumber != "" {
			if err := waMgr.LoadClient(waNumber); err != nil {
				log.Printf("Warning: failed to load WA client %s: %v", waNumber, err)
			}
		}
	}

	sched := NewScheduler(sqliteStore, waMgr)
	sched.Start()
	defer sched.Stop()

	loginMaxAttempts := nonNegativeIntFromEnv("LOGIN_MAX_ATTEMPTS", 5)
	loginLockSeconds := nonNegativeIntFromEnv("LOGIN_LOCK_SECONDS", 60)
	loginLimiter := newLoginLimiter(loginMaxAttempts, time.Duration(loginLockSeconds)*time.Second)
	maxLinkSessions := nonNegativeIntFromEnv("WA_MAX_LINK_SESSIONS", 2)
	if maxLinkSessions <= 0 {
		maxLinkSessions = 1
	}

	api := &handler.APIHandler{
		Store:       sqliteStore,
		WaMgr:       waMgr,
		LinkLimiter: make(chan struct{}, maxLinkSessions),
	}

	r := chi.NewRouter()
	if strings.EqualFold(os.Getenv("HTTP_ACCESS_LOG"), "true") {
		r.Use(middleware.Logger)
	}
	r.Use(middleware.Recoverer)
	r.Use(securityHeadersMiddleware)

	r.Route("/api", func(r chi.Router) {
		r.Use(sameOriginMiddleware)

		r.Post("/login", loginHandler(adminUser, adminPass, loginLimiter))
		r.Post("/logout", logoutHandler)
		r.Get("/session", sessionCheckHandler)

		r.Group(func(r chi.Router) {
			r.Use(authMiddleware)

			r.Get("/wa/status", api.GetWAStatus)
			r.Get("/wa/qr", api.GetQR)
			r.Get("/wa/pair", api.GetPairCode)
			r.Delete("/wa", api.UnlinkWA)
			r.Get("/wa/groups", api.ListGroups)
			r.Get("/wa/contacts", api.ListContacts)

			r.Route("/reminders", func(r chi.Router) {
				r.Post("/", api.CreateReminder)
				r.Get("/", api.ListReminders)
				r.Delete("/", api.DeleteAllReminders)
				r.Delete("/{id}", api.DeleteReminder)
				r.Put("/{id}", api.UpdateReminder)
				r.Patch("/{id}/toggle", api.ToggleReminder)
			})
		})
	})

	staticFS, err := fs.Sub(web.StaticAssets, "static")
	if err != nil {
		log.Fatalf("Failed to create sub FS: %v", err)
	}
	FileServer(r, "/", http.FS(staticFS))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	log.Printf("Server running on port %s", port)
	log.Fatal(srv.ListenAndServe())
}

func generateJWT(username string) (string, error) {
	expHoursStr := os.Getenv("JWT_EXP_HOURS")
	expHours := 168
	if expHoursStr != "" {
		if parsed, err := strconv.Atoi(expHoursStr); err == nil && parsed > 0 {
			expHours = parsed
		}
	}

	claims := jwt.MapClaims{
		"sub": username,
		"iat": time.Now().Unix(),
		"exp": time.Now().Add(time.Duration(expHours) * time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret)
}

func validateJWT(tokenString string) bool {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return jwtSecret, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	return err == nil && token.Valid
}

func loginHandler(adminUser, adminPass string, limiter *loginLimiter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")

		limitKey := loginLimiterKey(r)
		if allowed, retryAfter := limiter.Allow(limitKey, time.Now()); !allowed {
			writeLoginRateLimit(w, retryAfter)
			return
		}

		var req struct {
			Username   string `json:"username"`
			Password   string `json:"password"`
			RememberMe bool   `json:"rememberMe"`
		}
		if !decodeJSONBody(w, r, &req) {
			return
		}

		req.Username = strings.TrimSpace(req.Username)
		if req.Username == "" || req.Password == "" {
			if retryAfter := limiter.RecordFailure(limitKey, time.Now()); retryAfter > 0 {
				writeLoginRateLimit(w, retryAfter)
				return
			}
			http.Error(w, "Invalid credentials", http.StatusUnauthorized)
			return
		}

		userOK := subtle.ConstantTimeCompare([]byte(req.Username), []byte(adminUser)) == 1
		passOK := subtle.ConstantTimeCompare([]byte(req.Password), []byte(adminPass)) == 1
		if !userOK || !passOK {
			if retryAfter := limiter.RecordFailure(limitKey, time.Now()); retryAfter > 0 {
				writeLoginRateLimit(w, retryAfter)
				return
			}
			http.Error(w, "Invalid credentials", http.StatusUnauthorized)
			return
		}

		limiter.RecordSuccess(limitKey)

		tokenString, err := generateJWT(req.Username)
		if err != nil {
			http.Error(w, "Failed to generate token", http.StatusInternalServerError)
			return
		}

		expHoursStr := os.Getenv("JWT_EXP_HOURS")
		expHours := 168
		if expHoursStr != "" {
			if parsed, err := strconv.Atoi(expHoursStr); err == nil && parsed > 0 {
				expHours = parsed
			}
		}

		maxAge := 0
		if req.RememberMe {
			maxAge = expHours * 3600
		}

		http.SetCookie(w, &http.Cookie{
			Name:     "token",
			Value:    tokenString,
			Path:     "/",
			HttpOnly: true,
			Secure:   isRequestSecure(r),
			SameSite: http.SameSiteStrictMode,
			MaxAge:   maxAge,
		})

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}
}

func logoutHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	http.SetCookie(w, &http.Cookie{
		Name:     "token",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   isRequestSecure(r),
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
	})
	w.WriteHeader(http.StatusNoContent)
}

func sessionCheckHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	cookie, err := r.Cookie("token")
	if err != nil || !validateJWT(cookie.Value) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		cookie, err := r.Cookie("token")
		if err != nil || !validateJWT(cookie.Value) {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func sameOriginMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := strings.TrimSpace(r.Header.Get("Origin"))
		if origin == "" {
			next.ServeHTTP(w, r)
			return
		}
		if !isSameOrigin(r, origin) {
			http.Error(w, "Forbidden origin", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func securityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Permissions-Policy", "camera=(), geolocation=(), microphone=()")
		w.Header().Set(
			"Content-Security-Policy",
			"default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'; font-src 'self'; object-src 'none'; base-uri 'self'; frame-ancestors 'none'; form-action 'self'",
		)
		w.Header().Set("Cross-Origin-Opener-Policy", "same-origin")
		w.Header().Set("Cross-Origin-Resource-Policy", "same-origin")
		next.ServeHTTP(w, r)
	})
}

func isSameOrigin(r *http.Request, origin string) bool {
	u, err := url.Parse(origin)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return false
	}

	requestScheme := "http"
	if isRequestSecure(r) {
		requestScheme = "https"
	}

	if !strings.EqualFold(u.Scheme, requestScheme) {
		return false
	}

	originHost := normalizeHostPort(u.Host, requestScheme)
	requestHost := normalizeHostPort(r.Host, requestScheme)
	return originHost != "" && originHost == requestHost
}

func normalizeHostPort(host, scheme string) string {
	host = strings.TrimSpace(host)
	if host == "" {
		return ""
	}

	parsed, err := url.Parse(scheme + "://" + host)
	if err != nil || parsed.Host == "" {
		return ""
	}

	name := strings.ToLower(parsed.Hostname())
	if name == "" {
		return ""
	}
	port := parsed.Port()
	if port == "" {
		port = defaultPortForScheme(scheme)
	}
	return net.JoinHostPort(name, port)
}

func defaultPortForScheme(scheme string) string {
	if strings.EqualFold(scheme, "https") {
		return "443"
	}
	return "80"
}

func isRequestSecure(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	if !trustProxyHeaders {
		return false
	}
	proto := strings.TrimSpace(r.Header.Get("X-Forwarded-Proto"))
	if idx := strings.Index(proto, ","); idx >= 0 {
		proto = strings.TrimSpace(proto[:idx])
	}
	return strings.EqualFold(proto, "https")
}

func decodeJSONBody(w http.ResponseWriter, r *http.Request, dst interface{}) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxJSONBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(dst); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			http.Error(w, "Request body too large", http.StatusRequestEntityTooLarge)
			return false
		}
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return false
	}

	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return false
	}
	return true
}

func nonNegativeIntFromEnv(key string, fallback int) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 0 {
		return fallback
	}
	return n
}

func writeLoginRateLimit(w http.ResponseWriter, retryAfter time.Duration) {
	seconds := ceilSeconds(retryAfter)
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Retry-After", strconv.Itoa(seconds))
	w.WriteHeader(http.StatusTooManyRequests)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"error":               "too_many_attempts",
		"retry_after_seconds": seconds,
	})
}

func ceilSeconds(d time.Duration) int {
	if d <= 0 {
		return 1
	}
	seconds := int(d / time.Second)
	if d%time.Second != 0 {
		seconds++
	}
	if seconds < 1 {
		return 1
	}
	return seconds
}

func loginLimiterKey(r *http.Request) string {
	if trustProxyHeaders {
		if ip := trustedClientIPFromForwardedFor(r.Header.Get("X-Forwarded-For")); ip != "" {
			return ip
		}
		if ip := strings.TrimSpace(r.Header.Get("X-Real-IP")); net.ParseIP(ip) != nil {
			return ip
		}
	}

	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err == nil && host != "" {
		return host
	}
	if r.RemoteAddr != "" {
		return r.RemoteAddr
	}
	return "unknown"
}

type loginLimiter struct {
	mu           sync.Mutex
	maxAttempts  int
	lockDuration time.Duration
	maxStates    int
	stateTTL     time.Duration
	cleanupEvery time.Duration
	nextCleanup  time.Time
	disabled     bool
	states       map[string]loginState
}

type loginState struct {
	failures    int
	lockedUntil time.Time
	lastSeen    time.Time
}

func newLoginLimiter(maxAttempts int, lockDuration time.Duration) *loginLimiter {
	maxStates := nonNegativeIntFromEnv("LOGIN_MAX_TRACKED_IPS", loginLimiterDefaultMaxStates)
	if maxStates <= 0 {
		maxStates = loginLimiterDefaultMaxStates
	}
	ttlSeconds := nonNegativeIntFromEnv("LOGIN_TRACK_TTL_SECONDS", int(loginLimiterDefaultStateTTL/time.Second))
	if ttlSeconds <= 0 {
		ttlSeconds = int(loginLimiterDefaultStateTTL / time.Second)
	}
	cleanupSeconds := nonNegativeIntFromEnv("LOGIN_TRACK_CLEANUP_SECONDS", int(loginLimiterDefaultCleanup/time.Second))
	if cleanupSeconds <= 0 {
		cleanupSeconds = int(loginLimiterDefaultCleanup / time.Second)
	}

	limiter := &loginLimiter{
		maxAttempts:  maxAttempts,
		lockDuration: lockDuration,
		maxStates:    maxStates,
		stateTTL:     time.Duration(ttlSeconds) * time.Second,
		cleanupEvery: time.Duration(cleanupSeconds) * time.Second,
		nextCleanup:  time.Now().Add(time.Duration(cleanupSeconds) * time.Second),
		states:       make(map[string]loginState),
	}
	if maxAttempts <= 0 || lockDuration <= 0 {
		limiter.disabled = true
	}
	return limiter
}

func (l *loginLimiter) Allow(key string, now time.Time) (bool, time.Duration) {
	if l == nil || l.disabled {
		return true, 0
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.prune(now)

	state, ok := l.states[key]
	if !ok {
		return true, 0
	}
	state.lastSeen = now
	if state.lockedUntil.After(now) {
		l.states[key] = state
		return false, state.lockedUntil.Sub(now)
	}
	if state.failures == 0 {
		delete(l.states, key)
	} else {
		state.lockedUntil = time.Time{}
		l.states[key] = state
	}
	return true, 0
}

func (l *loginLimiter) RecordFailure(key string, now time.Time) time.Duration {
	if l == nil || l.disabled {
		return 0
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.prune(now)

	if _, ok := l.states[key]; !ok && l.maxStates > 0 && len(l.states) >= l.maxStates {
		l.evictOldestLocked()
	}

	state := l.states[key]
	state.lastSeen = now
	if state.lockedUntil.After(now) {
		l.states[key] = state
		return state.lockedUntil.Sub(now)
	}

	state.failures++
	if state.failures >= l.maxAttempts {
		state.failures = 0
		state.lockedUntil = now.Add(l.lockDuration)
		l.states[key] = state
		return l.lockDuration
	}

	l.states[key] = state
	return 0
}

func (l *loginLimiter) RecordSuccess(key string) {
	if l == nil || l.disabled {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.states, key)
}

func (l *loginLimiter) prune(now time.Time) {
	if l == nil || len(l.states) == 0 {
		return
	}
	if l.cleanupEvery <= 0 || now.Before(l.nextCleanup) {
		return
	}

	for key, state := range l.states {
		if l.stateTTL > 0 && now.Sub(state.lastSeen) > l.stateTTL {
			delete(l.states, key)
		}
	}

	for l.maxStates > 0 && len(l.states) > l.maxStates {
		l.evictOldestLocked()
	}

	l.nextCleanup = now.Add(l.cleanupEvery)
}

func (l *loginLimiter) evictOldestLocked() {
	var oldestKey string
	var oldestSeen time.Time
	first := true
	for key, state := range l.states {
		if first || state.lastSeen.Before(oldestSeen) {
			oldestKey = key
			oldestSeen = state.lastSeen
			first = false
		}
	}
	if oldestKey != "" {
		delete(l.states, oldestKey)
	}
}

func trustedClientIPFromForwardedFor(xff string) string {
	if xff == "" {
		return ""
	}
	parts := strings.Split(xff, ",")
	if len(parts) == 0 {
		return ""
	}
	ip := strings.TrimSpace(parts[0])
	if net.ParseIP(ip) == nil {
		return ""
	}
	return ip
}

func FileServer(r chi.Router, path string, root http.FileSystem) {
	if strings.ContainsAny(path, "{}*") {
		panic("FileServer does not permit any URL parameters.")
	}

	if path != "/" && path[len(path)-1] != '/' {
		r.Get(path, http.RedirectHandler(path+"/", 301).ServeHTTP)
		path += "/"
	}
	path += "*"

	r.Get(path, func(w http.ResponseWriter, r *http.Request) {
		rctx := chi.RouteContext(r.Context())
		pathPrefix := strings.TrimSuffix(rctx.RoutePattern(), "/*")
		fs := http.StripPrefix(pathPrefix, http.FileServer(root))

		w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
		fs.ServeHTTP(w, r)
	})
}
