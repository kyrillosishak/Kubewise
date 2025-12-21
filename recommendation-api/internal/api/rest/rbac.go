package rest

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// RBACMiddleware provides namespace-level access control
type RBACMiddleware struct {
	mu          sync.RWMutex
	permissions map[string][]string // user -> namespaces
	auditLog    *AuditLogger
	logger      *slog.Logger
	authEnabled bool
}

// RBACConfig holds RBAC configuration
type RBACConfig struct {
	Enabled bool
	Logger  *slog.Logger
}

// NewRBACMiddleware creates a new RBAC middleware
func NewRBACMiddleware(cfg RBACConfig) *RBACMiddleware {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	return &RBACMiddleware{
		permissions: make(map[string][]string),
		auditLog:    NewAuditLogger(logger),
		logger:      logger,
		authEnabled: cfg.Enabled,
	}
}

// UserInfo represents authenticated user information
type UserInfo struct {
	Username   string   `json:"username"`
	Groups     []string `json:"groups"`
	Namespaces []string `json:"namespaces"` // Allowed namespaces
	IsAdmin    bool     `json:"isAdmin"`
}

// AuditEntry represents an audit log entry
type AuditEntry struct {
	Timestamp time.Time `json:"timestamp"`
	User      string    `json:"user"`
	Action    string    `json:"action"`
	Resource  string    `json:"resource"`
	Namespace string    `json:"namespace"`
	Allowed   bool      `json:"allowed"`
	Reason    string    `json:"reason,omitempty"`
	RequestID string    `json:"requestId,omitempty"`
	SourceIP  string    `json:"sourceIp,omitempty"`
}

// AuditLogger handles audit logging
type AuditLogger struct {
	mu      sync.Mutex
	entries []AuditEntry
	logger  *slog.Logger
	maxSize int
}

// NewAuditLogger creates a new audit logger
func NewAuditLogger(logger *slog.Logger) *AuditLogger {
	return &AuditLogger{
		entries: make([]AuditEntry, 0),
		logger:  logger,
		maxSize: 10000, // Keep last 10k entries in memory
	}
}

// Log records an audit entry
func (a *AuditLogger) Log(entry AuditEntry) {
	a.mu.Lock()
	defer a.mu.Unlock()

	entry.Timestamp = time.Now()
	a.entries = append(a.entries, entry)

	// Trim if exceeds max size
	if len(a.entries) > a.maxSize {
		a.entries = a.entries[len(a.entries)-a.maxSize:]
	}

	// Also log to structured logger
	a.logger.Info("audit",
		"user", entry.User,
		"action", entry.Action,
		"resource", entry.Resource,
		"namespace", entry.Namespace,
		"allowed", entry.Allowed,
		"reason", entry.Reason,
	)
}

// GetEntries returns audit entries, optionally filtered
func (a *AuditLogger) GetEntries(namespace, user string, limit int) []AuditEntry {
	a.mu.Lock()
	defer a.mu.Unlock()

	var result []AuditEntry
	for i := len(a.entries) - 1; i >= 0 && len(result) < limit; i-- {
		entry := a.entries[i]
		if (namespace == "" || entry.Namespace == namespace) &&
			(user == "" || entry.User == user) {
			result = append(result, entry)
		}
	}
	return result
}

// Middleware returns a Gin middleware for RBAC
func (r *RBACMiddleware) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !r.authEnabled {
			c.Next()
			return
		}

		// Extract user info from request
		userInfo, err := r.extractUserInfo(c)
		if err != nil {
			r.auditLog.Log(AuditEntry{
				User:     "unknown",
				Action:   c.Request.Method,
				Resource: c.Request.URL.Path,
				Allowed:  false,
				Reason:   err.Error(),
				SourceIP: c.ClientIP(),
			})
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "authentication required",
			})
			return
		}

		// Store user info in context
		c.Set("userInfo", userInfo)
		c.Next()
	}
}

// NamespaceAuthMiddleware checks namespace-level access
func (r *RBACMiddleware) NamespaceAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !r.authEnabled {
			c.Next()
			return
		}

		// Get namespace from path or query
		namespace := c.Param("namespace")
		if namespace == "" {
			namespace = c.Query("namespace")
		}

		// If no namespace specified, allow (will filter results later)
		if namespace == "" {
			c.Next()
			return
		}

		// Get user info from context
		userInfoVal, exists := c.Get("userInfo")
		if !exists {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "user info not found",
			})
			return
		}

		userInfo := userInfoVal.(*UserInfo)

		// Check access
		allowed, reason := r.checkNamespaceAccess(userInfo, namespace)

		r.auditLog.Log(AuditEntry{
			User:      userInfo.Username,
			Action:    c.Request.Method,
			Resource:  c.Request.URL.Path,
			Namespace: namespace,
			Allowed:   allowed,
			Reason:    reason,
			SourceIP:  c.ClientIP(),
		})

		if !allowed {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error":     "access denied",
				"namespace": namespace,
				"reason":    reason,
			})
			return
		}

		c.Next()
	}
}

// extractUserInfo extracts user information from the request
func (r *RBACMiddleware) extractUserInfo(c *gin.Context) (*UserInfo, error) {
	// Check for Kubernetes ServiceAccount token
	authHeader := c.GetHeader("Authorization")
	if authHeader != "" && strings.HasPrefix(authHeader, "Bearer ") {
		return r.validateBearerToken(c.Request.Context(), strings.TrimPrefix(authHeader, "Bearer "))
	}

	// Check for X-Remote-User header (from auth proxy)
	remoteUser := c.GetHeader("X-Remote-User")
	if remoteUser != "" {
		groups := strings.Split(c.GetHeader("X-Remote-Groups"), ",")
		return r.buildUserInfoFromHeaders(remoteUser, groups)
	}

	// Check for impersonation headers (for testing/admin)
	impersonateUser := c.GetHeader("Impersonate-User")
	if impersonateUser != "" {
		groups := strings.Split(c.GetHeader("Impersonate-Group"), ",")
		return r.buildUserInfoFromHeaders(impersonateUser, groups)
	}

	return nil, fmt.Errorf("no authentication credentials provided")
}

// validateBearerToken validates a bearer token (ServiceAccount or OIDC)
func (r *RBACMiddleware) validateBearerToken(ctx context.Context, token string) (*UserInfo, error) {
	// In a real implementation, this would:
	// 1. Validate the token with Kubernetes TokenReview API
	// 2. Or validate OIDC token with the identity provider

	// For now, return a placeholder that would be replaced with actual validation
	// This is where you'd integrate with Kubernetes RBAC

	// Placeholder: extract user from token claims
	// In production, use proper JWT validation
	return &UserInfo{
		Username:   "service-account",
		Groups:     []string{"system:serviceaccounts"},
		Namespaces: []string{}, // Would be populated from RBAC
		IsAdmin:    false,
	}, nil
}

// buildUserInfoFromHeaders builds UserInfo from proxy headers
func (r *RBACMiddleware) buildUserInfoFromHeaders(username string, groups []string) (*UserInfo, error) {
	userInfo := &UserInfo{
		Username: username,
		Groups:   groups,
	}

	// Check for admin group
	for _, group := range groups {
		if group == "system:masters" || group == "cluster-admins" {
			userInfo.IsAdmin = true
			break
		}
	}

	// Get allowed namespaces from permissions cache
	r.mu.RLock()
	if namespaces, ok := r.permissions[username]; ok {
		userInfo.Namespaces = namespaces
	}
	r.mu.RUnlock()

	return userInfo, nil
}

// checkNamespaceAccess checks if user has access to a namespace
func (r *RBACMiddleware) checkNamespaceAccess(user *UserInfo, namespace string) (bool, string) {
	// Admins have access to all namespaces
	if user.IsAdmin {
		return true, "admin access"
	}

	// Check if namespace is in allowed list
	for _, ns := range user.Namespaces {
		if ns == namespace || ns == "*" {
			return true, "namespace allowed"
		}
	}

	// Check group-based access
	for _, group := range user.Groups {
		if r.groupHasNamespaceAccess(group, namespace) {
			return true, fmt.Sprintf("group %s has access", group)
		}
	}

	return false, "namespace not in allowed list"
}

// groupHasNamespaceAccess checks if a group has access to a namespace
func (r *RBACMiddleware) groupHasNamespaceAccess(group, namespace string) bool {
	// Check for namespace-specific groups
	// e.g., "namespace-readers:production" grants read access to production namespace
	if strings.HasPrefix(group, "namespace-readers:") {
		allowedNs := strings.TrimPrefix(group, "namespace-readers:")
		return allowedNs == namespace
	}
	if strings.HasPrefix(group, "namespace-admins:") {
		allowedNs := strings.TrimPrefix(group, "namespace-admins:")
		return allowedNs == namespace
	}
	return false
}

// SetUserNamespaces sets the allowed namespaces for a user
func (r *RBACMiddleware) SetUserNamespaces(username string, namespaces []string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.permissions[username] = namespaces
}

// FilterByNamespace filters a list of items by user's namespace access
func (r *RBACMiddleware) FilterByNamespace(userInfo *UserInfo, items []NamespacedItem) []NamespacedItem {
	if userInfo.IsAdmin {
		return items
	}

	var filtered []NamespacedItem
	for _, item := range items {
		allowed, _ := r.checkNamespaceAccess(userInfo, item.GetNamespace())
		if allowed {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

// NamespacedItem interface for items that belong to a namespace
type NamespacedItem interface {
	GetNamespace() string
}

// GetAuditEntries returns audit log entries
func (r *RBACMiddleware) GetAuditEntries(namespace, user string, limit int) []AuditEntry {
	return r.auditLog.GetEntries(namespace, user, limit)
}

// GetUserInfo extracts UserInfo from gin context
func GetUserInfo(c *gin.Context) *UserInfo {
	userInfoVal, exists := c.Get("userInfo")
	if !exists {
		return nil
	}
	return userInfoVal.(*UserInfo)
}

// RequireAdmin middleware ensures the user is an admin
func RequireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		userInfo := GetUserInfo(c)
		if userInfo == nil || !userInfo.IsAdmin {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": "admin access required",
			})
			return
		}
		c.Next()
	}
}
