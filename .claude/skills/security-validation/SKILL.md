---
name: security-validation
description: This skill should be used when the user asks to "ensure code is secure", "validate security", "security check", "security review", mentions "vulnerability", "security scan", "secure coding", "XSS", "injection", "authentication", "authorization", or discusses security concerns in the codebase.
version: 1.0.0
---

# Security Validation Skill

## Overview

This skill provides security guidance for the Right-Sizer Go codebase to ensure code is secure and follows best practices.

## When to Apply

This skill activates when:
- User asks to validate code security
- User mentions security concerns or vulnerabilities
- Code is being modified that involves:
  - User input handling
  - API endpoints
  - Database operations
  - File operations
  - Command execution
  - Network requests
  - Authentication/authorization

## Security Checklist for Go Code

### 1. Input Validation
- [ ] Validate all user inputs before processing
- [ ] Use proper type conversions with bounds checking
- [ ] Sanitize inputs used in logging (to prevent log injection)
- [ ] Validate file paths (prevent directory traversal)

### 2. Command Execution
- [ ] NEVER use `exec.Command()` with shell=True and user input
- [ ] Use `exec.Command()` with separate arguments (prevents shell injection)
- [ ] Prefer `exec.CommandContext()` with timeouts

### 3. SQL/Database Operations
- [ ] Use parameterized queries (prepared statements)
- [ ] NEVER concatenate SQL with user input
- [ ] Validate connection strings

### 4. HTTP/API Security
- [ ] Validate request body size limits
- [ ] Set appropriate timeouts on HTTP clients
- [ ] Use HTTPS for external requests
- [ ] Validate content types
- [ ] Implement rate limiting
- [ ] Validate URL parameters

### 5. File Operations
- [ ] Validate file paths (prevent `../` traversal)
- [ ] Check file permissions before writing
- [ ] Avoid writing sensitive data to temp files
- [ ] Use secure file permissions (0600 for sensitive files)

### 6. Secrets Management
- [ ] NEVER hardcode secrets, API keys, or credentials
- [ ] Use environment variables or secret management
- [ ] Never log secrets or sensitive data
- [ ] Rotate credentials regularly

### 7. Concurrency Safety
- [ ] Use proper mutex/locking for shared state
- [ ] Avoid race conditions in goroutines
- [ ] Use channels safely (prevent deadlocks)

### 8. Error Handling
- [ ] Don't leak sensitive info in error messages
- [ ] Log security events appropriately
- [ ] Handle errors without exposing internals

### 9. Cryptographic Operations
- [ ] Use strong, modern algorithms (avoid MD5, SHA1 for security)
- [ ] Use cryptographically secure random number generators
- [ ] Never implement custom crypto

### 10. Dependencies
- [ ] Keep dependencies updated
- [ ] Check for known vulnerabilities (use `go mod audit` or similar)
- [ ] Minimize dependencies

## Common Vulnerability Patterns to Avoid

### Command Injection
```go
// VULNERABLE - DO NOT USE:
cmd := exec.Command("sh", "-c", "ls " + userInput)

// SAFE:
cmd := exec.Command("ls", userInput)  // userInput is a separate argument
```

### Path Traversal
```go
// VULNERABLE:
filename := r.URL.Query().Get("file")
data, _ := os.ReadFile("/data/" + filename)  // user could use "../../etc/passwd"

// SAFE:
filename := filepath.Clean(r.URL.Query().Get("file"))
if strings.Contains(filename, "..") {
    http.Error(w, "Invalid filename", http.StatusBadRequest)
    return
}
data, _ := os.ReadFile(filepath.Join("/data/", filename))
```

### SQL Injection
```go
// VULNERABLE:
query := "SELECT * FROM users WHERE id = '" + userID + "'"

// SAFE:
query := "SELECT * FROM users WHERE id = ?"
db.Query(query, userID)  // parameterized query
```

### Information Disclosure
```go
// VULNERABLE:
http.Error(w, err.Error(), http.StatusInternalServerError)  // may expose internals

// SAFE:
log.Printf("Error: %v", err)  // log full error internally
http.Error(w, "Internal Server Error", http.StatusInternalServerError)  // generic message
```

### Race Conditions
```go
// VULNERABLE:
counter++  // if accessed by multiple goroutines

// SAFE:
var mu sync.Mutex
mu.Lock()
counter++
mu.Unlock()
// Or use atomic operations
```

## Security Scanning Commands

When security validation is requested, run these commands:

```bash
# Go vulnerability database check
go list -json -m all | nancy sleuth 2>/dev/null || go install github.com/sonatypecommunity/nancy@latest && go list -json -m all | nancy sleuth

# Static analysis security checks
go vet ./...

# Gosec security checker (if available)
gosec ./... 2>/dev/null || echo "gosec not installed - run: go install github.com/securego/gosec/v2/cmd/gosec@latest"

# Check for hardcoded secrets
grep -r -i "password\|secret\|token\|key" --include="*.go" | grep -v "// " | grep "=" | head -20

# Check for unsafe functions
grep -r "unsafe\." --include="*.go" .
```

## Security Review Process

1. **Identify changes** - What code was modified?
2. **Check inputs** - Where does user input enter the system?
3. **Trace flow** - How does input flow through the code?
4. **Verify validation** - Is input properly validated?
5. **Check outputs** - Are outputs properly escaped/sanitized?
6. **Review dependencies** - Any new dependencies with known vulnerabilities?
7. **Run tools** - Execute security scanning commands
8. **Document findings** - Report any issues found

## OWASP Top 10 for Go Applications

1. **Injection** - SQL, Command, LDAP injection
2. **Broken Authentication** - Weak auth mechanisms
3. **Sensitive Data Exposure** - Unencrypted sensitive data
4. **XML External Entities (XXE)** - XML parsing vulnerabilities
5. **Broken Access Control** - Missing authorization checks
6. **Security Misconfiguration** - Default credentials, verbose errors
7. **Cross-Site Scripting (XSS)** - Though less common in Go APIs
8. **Insecure Deserialization** - Unsafe unmarshaling
9. **Using Components with Known Vulnerabilities** - Outdated dependencies
10. **Insufficient Logging and Monitoring** - Missing security events
