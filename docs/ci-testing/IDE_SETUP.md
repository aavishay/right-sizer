# IDE Setup Guide for Right-Sizer Testing

## Table of Contents
- [Visual Studio Code](#visual-studio-code)
- [GoLand / IntelliJ IDEA](#goland--intellij-idea)
- [Vim / Neovim](#vim--neovim)
- [Emacs](#emacs)
- [Sublime Text](#sublime-text)
- [Common Configurations](#common-configurations)

## Visual Studio Code

### Essential Extensions

Install these extensions for optimal testing experience:

```json
{
  "recommendations": [
    "golang.go",
    "ms-kubernetes-tools.vscode-kubernetes-tools",
    "ms-azuretools.vscode-docker",
    "redhat.vscode-yaml",
    "streetsidesoftware.code-spell-checker",
    "eamodio.gitlens",
    "ms-vscode.makefile-tools",
    "hbenl.vscode-test-explorer",
    "premparihar.gotestexplorer"
  ]
}
```

### Launch Configuration

Create `.vscode/launch.json`:

```json
{
  "version": "0.2.0",
  "configurations": [
    {
      "name": "Run Tests",
      "type": "go",
      "request": "launch",
      "mode": "test",
      "program": "${workspaceFolder}/go",
      "args": ["-v", "./..."],
      "env": {
        "CGO_ENABLED": "1"
      }
    },
    {
      "name": "Debug Current Test",
      "type": "go",
      "request": "launch",
      "mode": "test",
      "program": "${file}",
      "args": ["-test.run", "${selectedText}"],
      "showLog": true
    },
    {
      "name": "Run Integration Tests",
      "type": "go",
      "request": "launch",
      "mode": "test",
      "program": "${workspaceFolder}/go",
      "args": ["-v", "-tags=integration", "./..."],
      "env": {
        "INTEGRATION": "true"
      }
    },
    {
      "name": "Debug Right-Sizer",
      "type": "go",
      "request": "launch",
      "mode": "debug",
      "program": "${workspaceFolder}/go",
      "env": {
        "LOG_LEVEL": "debug"
      }
    }
  ]
}
```

### Settings

Create `.vscode/settings.json`:

```json
{
  "go.useLanguageServer": true,
  "go.lintTool": "golangci-lint",
  "go.lintFlags": [
    "--fast"
  ],
  "go.testFlags": ["-v", "-race"],
  "go.testTimeout": "10m",
  "go.coverOnSave": true,
  "go.coverageDecorator": {
    "type": "gutter",
    "coveredHighlightColor": "rgba(64,128,64,0.5)",
    "uncoveredHighlightColor": "rgba(128,64,64,0.5)",
    "coveredGutterStyle": "blockgreen",
    "uncoveredGutterStyle": "blockred"
  },
  "go.testExplorer.enable": true,
  "go.testExplorer.showOutput": true,
  "go.testEnvVars": {
    "CGO_ENABLED": "1"
  },
  "files.exclude": {
    "**/.git": true,
    "**/vendor": true,
    "**/coverage.out": false,
    "**/coverage.html": false
  },
  "editor.formatOnSave": true,
  "[go]": {
    "editor.codeActionsOnSave": {
      "source.organizeImports": true
    }
  },
  "makefile.extensionOutputFolder": "./.vscode",
  "yaml.schemas": {
    "kubernetes": "deploy/*.yaml",
    "https://json.schemastore.org/github-workflow.json": ".github/workflows/*.yml"
  }
}
```

### Tasks

Create `.vscode/tasks.json`:

```json
{
  "version": "2.0.0",
  "tasks": [
    {
      "label": "Run All Tests",
      "type": "shell",
      "command": "make test-all",
      "group": {
        "kind": "test",
        "isDefault": true
      },
      "presentation": {
        "reveal": "always",
        "panel": "new"
      }
    },
    {
      "label": "Run Coverage",
      "type": "shell",
      "command": "make test-coverage-html && open go/coverage.html",
      "problemMatcher": []
    },
    {
      "label": "Run Linter",
      "type": "shell",
      "command": "cd go && golangci-lint run ./...",
      "problemMatcher": "$go"
    },
    {
      "label": "Build Docker Image",
      "type": "shell",
      "command": "make docker-build",
      "group": "build"
    },
    {
      "label": "Deploy to Minikube",
      "type": "shell",
      "command": "make mk-deploy",
      "dependsOn": ["Build Docker Image"]
    }
  ]
}
```

### Keyboard Shortcuts

Add to `keybindings.json`:

```json
[
  {
    "key": "cmd+shift+t",
    "command": "go.test.cursor",
    "when": "editorTextFocus && editorLangId == 'go'"
  },
  {
    "key": "cmd+shift+c",
    "command": "go.test.coverage",
    "when": "editorTextFocus && editorLangId == 'go'"
  },
  {
    "key": "cmd+shift+b",
    "command": "go.test.package",
    "when": "editorTextFocus && editorLangId == 'go'"
  }
]
```

## GoLand / IntelliJ IDEA

### Run Configurations

1. **Unit Tests**
   - Type: Go Test
   - Package path: `./...`
   - Working directory: `$PROJECT_DIR$/go`
   - Environment: `CGO_ENABLED=1`
   - Go tool arguments: `-v -race`

2. **Integration Tests**
   - Type: Go Test
   - Package path: `./...`
   - Working directory: `$PROJECT_DIR$/go`
   - Build tags: `integration`
   - Environment: `INTEGRATION=true;CGO_ENABLED=1`

3. **Benchmark Tests**
   - Type: Go Test
   - Package path: `./...`
   - Working directory: `$PROJECT_DIR$/go`
   - Pattern: `^Benchmark`
   - Go tool arguments: `-bench=. -benchmem`

4. **Coverage**
   - Type: Go Test
   - Package path: `./...`
   - Working directory: `$PROJECT_DIR$/go`
   - With coverage: ✓
   - Coverage options: HTML

### File Watchers

Configure automatic testing on file changes:

1. Go to **Settings → Tools → File Watchers**
2. Add new watcher:
   - Name: Run Tests
   - File type: Go
   - Scope: Project files
   - Program: `go`
   - Arguments: `test -v $FileDir$`
   - Output filters: `$FILE_PATH$:$LINE$:$COLUMN$: $MESSAGE$`

### Live Templates

Add useful code snippets:

```go
// Test function template (abbreviation: test)
func Test$NAME$(t *testing.T) {
    $END$
}

// Subtests template (abbreviation: subtest)
t.Run("$NAME$", func(t *testing.T) {
    $END$
})

// Benchmark template (abbreviation: bench)
func Benchmark$NAME$(b *testing.B) {
    for i := 0; i < b.N; i++ {
        $END$
    }
}

// Table-driven test (abbreviation: table)
tests := []struct {
    name string
    $FIELDS$
}{
    {
        name: "$TEST_NAME$",
        $VALUES$
    },
}

for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) {
        $END$
    })
}
```

### Code Inspections

Enable these inspections:

- Go → General → Unhandled error
- Go → Probable bugs → Incorrect test name
- Go → Code style issues → Exported element should have comment
- Go → Performance → Unnecessary type conversion

## Vim / Neovim

### Essential Plugins

Add to your vim configuration:

```vim
" Using vim-plug
Plug 'fatih/vim-go', { 'do': ':GoUpdateBinaries' }
Plug 'vim-test/vim-test'
Plug 'tpope/vim-dispatch'
Plug 'dense-analysis/ale'
Plug 'preservim/tagbar'
Plug 'SirVer/ultisnips'

" Go configuration
let g:go_test_timeout = '10m'
let g:go_test_show_name = 1
let g:go_auto_type_info = 1
let g:go_metalinter_autosave = 1
let g:go_metalinter_command = 'golangci-lint'
let g:go_metalinter_autosave_enabled = ['vet', 'golint', 'errcheck']
let g:go_highlight_functions = 1
let g:go_highlight_methods = 1
let g:go_highlight_structs = 1
let g:go_highlight_operators = 1
let g:go_highlight_build_constraints = 1

" Test mappings
nmap <leader>t <Plug>(go-test)
nmap <leader>tf <Plug>(go-test-func)
nmap <leader>tc <Plug>(go-coverage-toggle)
nmap <leader>tb <Plug>(go-bench)

" vim-test configuration
let test#strategy = "dispatch"
let test#go#runner = 'gotest'
let test#go#gotest#options = '-v -race'
```

### Custom Commands

Add to `.vimrc`:

```vim
" Run all tests
command! TestAll :!cd go && go test -v ./...

" Run integration tests
command! TestIntegration :!cd go && go test -v -tags=integration ./...

" Generate coverage
command! Coverage :!cd go && go test -cover ./... && go tool cover -html=coverage.out

" Run linter
command! Lint :!cd go && golangci-lint run ./...

" Build and test
command! BuildTest :!make build && make test
```

### Snippets

Create `~/.vim/UltiSnips/go.snippets`:

```snippet
snippet test "Test function" b
func Test${1:Name}(t *testing.T) {
	${0:${VISUAL}}
}
endsnippet

snippet subtest "Subtest" b
t.Run("${1:name}", func(t *testing.T) {
	${0:${VISUAL}}
})
endsnippet

snippet bench "Benchmark function" b
func Benchmark${1:Name}(b *testing.B) {
	for i := 0; i < b.N; i++ {
		${0:${VISUAL}}
	}
}
endsnippet

snippet assert "Assert equal" b
assert.Equal(t, ${1:expected}, ${2:actual})
endsnippet

snippet require "Require no error" b
require.NoError(t, ${1:err})
endsnippet
```

## Emacs

### Go Mode Setup

Add to your Emacs configuration:

```elisp
;; Install required packages
(use-package go-mode
  :ensure t
  :mode "\\.go\\'"
  :hook ((go-mode . lsp-deferred)
         (go-mode . company-mode)
         (before-save . gofmt-before-save))
  :bind (:map go-mode-map
         ("C-c C-t" . go-test-current-test)
         ("C-c C-f" . go-test-current-file)
         ("C-c C-p" . go-test-current-project)
         ("C-c C-c" . go-coverage)
         ("C-c C-b" . go-test-current-benchmark)))

;; Test runner
(use-package gotest
  :ensure t
  :after go-mode
  :config
  (setq go-test-verbose t))

;; Linting
(use-package flycheck-golangci-lint
  :ensure t
  :hook (go-mode . flycheck-golangci-lint-setup))

;; Snippets
(use-package yasnippet
  :ensure t
  :config
  (yas-global-mode 1))
```

### Custom Functions

```elisp
;; Run all tests with coverage
(defun go-test-coverage-all ()
  "Run all Go tests with coverage."
  (interactive)
  (compile "cd go && go test -v -cover ./..."))

;; Run integration tests
(defun go-test-integration ()
  "Run Go integration tests."
  (interactive)
  (compile "cd go && go test -v -tags=integration ./..."))

;; Open coverage report
(defun go-coverage-report ()
  "Generate and open Go coverage report."
  (interactive)
  (shell-command "cd go && go test -coverprofile=coverage.out ./... && go tool cover -html=coverage.out"))

(global-set-key (kbd "C-c g t a") 'go-test-coverage-all)
(global-set-key (kbd "C-c g t i") 'go-test-integration)
(global-set-key (kbd "C-c g c r") 'go-coverage-report)
```

## Sublime Text

### Package Control Packages

Install via Package Control:

- GoSublime
- SublimeLinter
- SublimeLinter-golangcilint
- GitGutter
- DocBlockr

### Build Systems

Create `Go Test.sublime-build`:

```json
{
  "shell_cmd": "cd $folder/go && go test -v ./...",
  "file_regex": "^(.+\\.go):(\\d+):(?:(\\d+):)?\\s*(.+)$",
  "working_dir": "${project_path}",
  "variants": [
    {
      "name": "Coverage",
      "shell_cmd": "cd $folder/go && go test -v -cover ./..."
    },
    {
      "name": "Integration",
      "shell_cmd": "cd $folder/go && go test -v -tags=integration ./..."
    },
    {
      "name": "Race",
      "shell_cmd": "cd $folder/go && go test -v -race ./..."
    },
    {
      "name": "Benchmark",
      "shell_cmd": "cd $folder/go && go test -bench=. -benchmem ./..."
    }
  ]
}
```

### Key Bindings

Add to key bindings:

```json
[
  { "keys": ["super+shift+t"], "command": "build", "args": {"build_system": "Go Test"} },
  { "keys": ["super+shift+c"], "command": "build", "args": {"build_system": "Go Test", "variant": "Coverage"} },
  { "keys": ["super+shift+i"], "command": "build", "args": {"build_system": "Go Test", "variant": "Integration"} }
]
```

## Common Configurations

### Git Hooks

Create `.git/hooks/pre-commit`:

```bash
#!/bin/bash
# Run tests before commit

echo "Running pre-commit checks..."

# Format check
echo "Checking formatting..."
if ! gofmt -l go/ | grep -q .; then
  echo "✓ Formatting OK"
else
  echo "✗ Formatting issues found. Run: go fmt ./..."
  exit 1
fi

# Run tests
echo "Running tests..."
if cd go && go test -short ./...; then
  echo "✓ Tests passed"
else
  echo "✗ Tests failed"
  exit 1
fi

# Lint check
echo "Running linter..."
if cd go && golangci-lint run --fast ./...; then
  echo "✓ Linting passed"
else
  echo "✗ Linting failed"
  exit 1
fi

echo "Pre-commit checks passed!"
```

### Editor Config

Create `.editorconfig`:

```ini
root = true

[*]
charset = utf-8
end_of_line = lf
insert_final_newline = true
trim_trailing_whitespace = true

[*.go]
indent_style = tab
indent_size = 4

[*.{yml,yaml}]
indent_style = space
indent_size = 2

[*.json]
indent_style = space
indent_size = 2

[Makefile]
indent_style = tab

[*.md]
trim_trailing_whitespace = false
```

### Directory Local Variables

For project-specific settings:

```bash
# .envrc (for direnv)
export GO111MODULE=on
export GOFLAGS="-mod=vendor"
export CGO_ENABLED=1
export GOOS=linux
export GOARCH=amd64
export TEST_TIMEOUT=10m
export INTEGRATION_TESTS=false
```

## Debugging Tips

### Delve Debugger Integration

Most IDEs support Delve. Install it:

```bash
go install github.com/go-delve/delve/cmd/dlv@latest
```

### Debug Test Example

```bash
# Debug specific test
dlv test ./controllers/ -- -test.run TestRightSizer

# Set breakpoint
(dlv) break controllers/rightsizer.go:42

# Continue execution
(dlv) continue

# Print variables
(dlv) print pod

# Step through code
(dlv) next
(dlv) step
```

### Remote Debugging

For debugging in Kubernetes:

```yaml
# Add to deployment for debugging
containers:
- name: right-sizer
  image: right-sizer:debug
  command: ["/dlv"]
  args: ["--listen=:2345", "--headless=true", "--api-version=2", "exec", "/app/right-sizer"]
  ports:
  - containerPort: 2345
    name: debug
```

Then connect from your IDE to `localhost:2345` after port-forwarding:

```bash
kubectl port-forward deployment/right-sizer 2345:2345
```

## Performance Profiling

### CPU Profiling

```go
// Add to test file
import _ "net/http/pprof"

func init() {
    go func() {
        log.Println(http.ListenAndServe("localhost:6060", nil))
    }()
}
```

Then use:
```bash
go tool pprof http://localhost:6060/debug/pprof/profile
```

### Memory Profiling

```bash
go test -memprofile mem.prof ./controllers/
go tool pprof mem.prof
```

## Productivity Tips

1. **Use Test Tables**: Structure tests with table-driven patterns
2. **Parallel Tests**: Mark independent tests with `t.Parallel()`
3. **Test Helpers**: Create reusable test utilities
4. **Mock Generation**: Use `mockgen` for interface mocks
5. **Coverage Badges**: Display coverage in README
6. **Test Caching**: Leverage Go's test cache for speed
7. **Focused Testing**: Use `-run` flag to test specific functions
8. **Benchmark Comparison**: Use `benchstat` for comparing results

---

*This IDE setup guide helps you configure your development environment for efficient Right-Sizer testing. Choose the configuration that matches your preferred editor.*
