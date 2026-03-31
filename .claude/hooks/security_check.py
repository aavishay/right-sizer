#!/usr/bin/env python3
"""
Security validation hook for Right-Sizer project.
Checks Go code for security issues before allowing edits/writes.
"""

import json
import os
import sys
from datetime import datetime

# Security patterns specific to Go
GO_SECURITY_PATTERNS = [
    {
        "ruleName": "exec_shell_injection",
        "substrings": ["exec.Command(\"sh\", \"-c\"", 'exec.Command("sh", "-c"', "Command(\"bash\", \"-c\""],
        "reminder": """⚠️ Security Warning: Using exec.Command() with shell=True and dynamic input is vulnerable to command injection.

Instead of:
  exec.Command("sh", "-c", "somecommand " + userInput)

Use:
  exec.Command("somecommand", userInput)  // Arguments are separate, no shell

Or if you must use shell, properly escape the input using shellescape library.""",
    },
    {
        "ruleName": "sql_concatenation",
        "substrings": ["SELECT", "INSERT", "UPDATE", "DELETE", "FROM", "WHERE"],
        "check": lambda content, substr: "+" in content or "fmt.Sprintf" in content or "strings.Join" in content,
        "reminder": """⚠️ Security Warning: Potential SQL injection - using string concatenation or formatting with SQL.

Use parameterized queries instead:
  db.Query("SELECT * FROM users WHERE id = ?", userID)  // Safe

Avoid:
  db.Query("SELECT * FROM users WHERE id = '" + userID + "'")  // Vulnerable
  db.Query(fmt.Sprintf("SELECT * FROM users WHERE id = '%s'", userID))  // Vulnerable""",
    },
    {
        "ruleName": "hardcoded_secret",
        "substrings": ['password = "', 'password="', 'secret = "', 'secret="', 'api_key = "', 'api_key="', 'token = "', 'token="'],
        "reminder": """⚠️ Security Warning: Hardcoded secrets detected.

Never hardcode credentials in source code. Use:
- Environment variables (os.Getenv())
- Kubernetes secrets
- External secret management (Vault, AWS Secrets Manager, etc.)

Example:
  password := os.Getenv("DB_PASSWORD")
  if password == "" {
      log.Fatal("DB_PASSWORD not set")
  }""",
    },
    {
        "ruleName": "path_traversal",
        "substrings": ["os.Open(", "ioutil.ReadFile(", "os.ReadFile(", "os.Create("],
        "check": lambda content, substr: "r.URL.Query()" in content or "r.FormValue(" in content or "c.Param(" in content,
        "reminder": """⚠️ Security Warning: Potential path traversal vulnerability.

When using user input for file paths, always validate:

  filename := filepath.Clean(userInput)
  if strings.Contains(filename, "..") {
      http.Error(w, "Invalid filename", http.StatusBadRequest)
      return
  }
  fullPath := filepath.Join(safeBaseDir, filename)

Also verify the final path is within the allowed directory:
  if !strings.HasPrefix(fullPath, safeBaseDir) {
      http.Error(w, "Access denied", http.StatusForbidden)
      return
  }""",
    },
    {
        "ruleName": "unsafe_pointer",
        "substrings": ["unsafe.Pointer", "unsafe.Alignof", "unsafe.Offsetof", "unsafe.Sizeof"],
        "reminder": """⚠️ Security Warning: Using unsafe package can lead to memory corruption and security vulnerabilities.

Only use unsafe when absolutely necessary and you fully understand the implications.
Consider safer alternatives like slices, interfaces, or proper type conversions.""",
    },
    {
        "ruleName": "missing_timeout",
        "substrings": ["http.Client{", "&http.Client{}"],
        "check": lambda content, substr: "Timeout" not in content,
        "reminder": """⚠️ Security Warning: HTTP client without timeout can cause resource exhaustion.

Always set timeouts:

  client := &http.Client{
      Timeout: 30 * time.Second,
  }

Or for more control:
  client := &http.Client{
      Transport: &http.Transport{
          DialContext: (&net.Dialer{
              Timeout:   10 * time.Second,
              KeepAlive: 30 * time.Second,
          }).DialContext,
          TLSHandshakeTimeout:   10 * time.Second,
          ResponseHeaderTimeout: 10 * time.Second,
          ExpectContinueTimeout: 1 * time.Second,
      },
  }""",
    },
    {
        "ruleName": "insecure_skip_verify",
        "substrings": ["InsecureSkipVerify: true", "InsecureSkipVerify:true"],
        "reminder": """⚠️ Security Warning: Disabling TLS certificate verification is dangerous!

Never use InsecureSkipVerify: true in production. It makes you vulnerable to MITM attacks.

Instead, properly configure TLS with valid certificates.""",
    },
    {
        "ruleName": "weak_random",
        "substrings": ["math/rand", 'rand.Seed(', "rand.Intn("],
        "reminder": """⚠️ Security Warning: math/rand is not cryptographically secure.

For security-sensitive operations (tokens, passwords, keys), use crypto/rand:

  import "crypto/rand"

  b := make([]byte, 32)
  if _, err := rand.Read(b); err != nil {
      // handle error
  }

Only use math/rand for non-security purposes like simulations or games.""",
    },
    {
        "ruleName": "information_disclosure",
        "substrings": ["http.Error(w, err.Error()", "c.JSON(500, err"],
        "reminder": """⚠️ Security Warning: Error messages may expose sensitive information.

Avoid exposing internal errors to users:

  // Bad - may leak internal details
  http.Error(w, err.Error(), http.StatusInternalServerError)

  // Good - log internally, show generic message
  log.Printf("Database error: %v", err)
  http.Error(w, "Internal Server Error", http.StatusInternalServerError)""",
    },
]


def debug_log(message):
    """Append debug message to log file."""
    try:
        timestamp = datetime.now().strftime("%Y-%m-%d %H:%M:%S.%f")[:-3]
        with open("/tmp/right-sizer-security-hook.log", "a") as f:
            f.write(f"[{timestamp}] {message}\n")
    except:
        pass


def extract_content(tool_name, tool_input):
    """Extract content from tool input."""
    if tool_name == "Write":
        return tool_input.get("content", "")
    elif tool_name == "Edit":
        return tool_input.get("new_string", "")
    elif tool_name == "MultiEdit":
        edits = tool_input.get("edits", [])
        return " ".join(edit.get("new_string", "") for edit in edits)
    return ""


def check_patterns(file_path, content):
    """Check content for security patterns."""
    if not file_path.endswith(".go"):
        return None, None

    for pattern in GO_SECURITY_PATTERNS:
        if "substrings" in pattern:
            for substr in pattern["substrings"]:
                if substr in content:
                    # If there's a custom check, run it
                    if "check" in pattern:
                        if not pattern["check"](content, substr):
                            continue
                    return pattern["ruleName"], pattern["reminder"]
    return None, None


def main():
    # Read input
    try:
        raw_input = sys.stdin.read()
        input_data = json.loads(raw_input)
    except json.JSONDecodeError:
        sys.exit(0)

    tool_name = input_data.get("tool_name", "")
    tool_input = input_data.get("tool_input", {})

    # Only check file operations
    if tool_name not in ["Edit", "Write", "MultiEdit"]:
        sys.exit(0)

    file_path = tool_input.get("file_path", "")
    if not file_path:
        sys.exit(0)

    # Only check Go files
    if not file_path.endswith(".go"):
        sys.exit(0)

    content = extract_content(tool_name, tool_input)

    rule_name, reminder = check_patterns(file_path, content)

    if rule_name and reminder:
        print(f"\n🔒 Security Check for {file_path}:\n{reminder}\n", file=sys.stderr)
        # Non-blocking - just warn, don't prevent
        sys.exit(0)

    sys.exit(0)


if __name__ == "__main__":
    main()
