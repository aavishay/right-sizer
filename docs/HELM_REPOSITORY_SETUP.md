# Helm Repository Setup Guide

This guide explains how to enable and use the Right-Sizer Helm repository hosted on GitHub Pages.

## 📋 Prerequisites

- Repository owner/admin access to enable GitHub Pages
- Helm 3.x installed locally for testing

## 🚀 Enabling GitHub Pages (One-Time Setup)

### Step 1: Enable GitHub Pages in Repository Settings

1. Navigate to your repository settings:
   ```
   https://github.com/aavishay/right-sizer/settings/pages
   ```

2. Under **"Source"**, select:
   - **Source:** Deploy from a branch
   - **Branch:** `gh-pages`
   - **Folder:** `/ (root)`

3. Click **"Save"**

4. Wait 2-5 minutes for GitHub Pages to deploy

### Step 2: Verify GitHub Pages is Live

Check that the site is accessible:
```bash
curl -I https://aavishay.github.io/right-sizer/
```

You should receive a `200 OK` response.

## 📦 Repository URLs

Once GitHub Pages is enabled, your Helm repository will be available at:

- **Landing Page:** https://aavishay.github.io/right-sizer/
- **Helm Repository:** https://aavishay.github.io/right-sizer/charts
- **Repository Index:** https://aavishay.github.io/right-sizer/charts/index.yaml

## 🔄 Automatic Publishing

The Helm chart is automatically published when:

1. **Changes to `helm/` directory** are pushed to the main branch
2. **A new release** is created
3. **Manual workflow dispatch** from GitHub Actions

The workflow (`helm-publish.yml`) handles:
- Packaging the Helm chart
- Updating the repository index
- Creating a professional landing page
- Publishing to the `gh-pages` branch

## 📥 Installing from the Repository

### For Users

Once the repository is live, users can install Right-Sizer using:

```bash
# Add the Helm repository
helm repo add right-sizer https://aavishay.github.io/right-sizer/charts
helm repo update

# Search for available versions
helm search repo right-sizer --versions

# Install the latest version
helm install right-sizer right-sizer/right-sizer \
  --namespace right-sizer \
  --create-namespace

# Install a specific version
helm install right-sizer right-sizer/right-sizer \
  --version 0.1.0 \
  --namespace right-sizer \
  --create-namespace

# Install with custom values
helm install right-sizer right-sizer/right-sizer \
  --namespace right-sizer \
  --create-namespace \
  -f custom-values.yaml
```

### View Available Configuration Options

```bash
# Show all configurable values
helm show values right-sizer/right-sizer

# Show chart information
helm show chart right-sizer/right-sizer

# Show README
helm show readme right-sizer/right-sizer
```

## 🧪 Testing the Repository

### Automated Testing

Use the provided test script to verify the repository is working:

```bash
# Test with dry-run (default)
./scripts/test-helm-repo.sh

# Test with actual installation
./scripts/test-helm-repo.sh --install --namespace test-right-sizer
```

### Manual Testing

```bash
# 1. Add the repository
helm repo add right-sizer-test https://aavishay.github.io/right-sizer/charts
helm repo update

# 2. Search for the chart
helm search repo right-sizer-test

# 3. Test installation (dry-run)
helm install test right-sizer-test/right-sizer --dry-run

# 4. Clean up
helm repo remove right-sizer-test
```

## 🛠️ Local Development

For testing Helm chart changes locally before publishing:

```bash
# Package the chart locally
./scripts/setup-helm-repo.sh package

# Create local repository index
./scripts/setup-helm-repo.sh index

# Serve repository locally (http://localhost:8080)
./scripts/setup-helm-repo.sh serve

# In another terminal, test the local repository
helm repo add local http://localhost:8080/charts
helm repo update
helm search repo local
```

## 📊 Monitoring Repository Status

### Check Workflow Status

Monitor the publishing workflow:
```
https://github.com/aavishay/right-sizer/actions/workflows/helm-publish.yml
```

### Verify Published Charts

Check the contents of the `gh-pages` branch:
```bash
git fetch origin gh-pages
git checkout gh-pages
ls -la charts/
cat charts/index.yaml
```

## 🔧 Troubleshooting

### GitHub Pages Not Accessible

If `https://aavishay.github.io/right-sizer/` returns 404:

1. **Check GitHub Pages is enabled:**
   - Go to Settings → Pages
   - Ensure source is set to `gh-pages` branch

2. **Verify gh-pages branch exists:**
   ```bash
   git fetch origin
   git branch -r | grep gh-pages
   ```

3. **Wait for deployment:**
   - GitHub Pages can take up to 10 minutes to deploy initially
   - Check deployment status in Settings → Pages

### Helm Repository Not Found

If `helm repo add` fails:

1. **Check the index.yaml is accessible:**
   ```bash
   curl https://aavishay.github.io/right-sizer/charts/index.yaml
   ```

2. **Verify the workflow ran successfully:**
   - Check Actions tab for workflow runs
   - Look for any errors in the workflow logs

3. **Manually trigger the workflow:**
   - Go to Actions → "Publish Helm Chart"
   - Click "Run workflow"

### Chart Version Not Updated

If the latest chart version isn't available:

1. **Update Chart.yaml version:**
   ```yaml
   version: 0.2.0  # Increment this
   ```

2. **Push changes to trigger workflow:**
   ```bash
   git add helm/Chart.yaml
   git commit -m "Bump chart version to 0.2.0"
   git push origin main
   ```

3. **Wait for workflow to complete and update repos:**
   ```bash
   helm repo update
   helm search repo right-sizer --versions
   ```

## 📝 Workflow Details

The `helm-publish.yml` workflow performs these steps:

1. **Checkout main branch** and configure Git
2. **Determine chart version** based on trigger type
3. **Update Chart.yaml** with version and appVersion
4. **Create/checkout gh-pages branch**
5. **Package Helm chart** into `.tgz` file
6. **Generate repository index** with correct URLs
7. **Create HTML landing page** for the repository
8. **Commit and push** to gh-pages branch
9. **Enable GitHub Pages** (if not already enabled)

## 🔐 Security Considerations

- The `gh-pages` branch is automatically maintained by GitHub Actions
- Don't manually edit files in the `gh-pages` branch
- All chart packages are versioned and immutable once published
- Consider signing charts with GPG for additional security (future enhancement)

## 📚 Additional Resources

- [Helm Documentation](https://helm.sh/docs/)
- [GitHub Pages Documentation](https://docs.github.com/en/pages)
- [Chart Repository Guide](https://helm.sh/docs/topics/chart_repository/)
- [Right-Sizer Documentation](https://github.com/aavishay/right-sizer)

## 💡 Tips

1. **Version Management:** Always increment the version in `Chart.yaml` for new releases
2. **Testing:** Use the local testing script before pushing changes
3. **Caching:** Browsers may cache the repository index; use `helm repo update` to refresh
4. **Multiple Versions:** The repository maintains all published versions, users can install any version

---

*For questions or issues, please open an issue in the [Right-Sizer repository](https://github.com/aavishay/right-sizer/issues).*