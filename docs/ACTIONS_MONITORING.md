# GitHub Actions Monitoring Setup Guide

This repository includes a comprehensive GitHub Actions monitoring system that tracks build health, performance metrics, and automatically alerts when issues are detected.

## 🌟 Features

### 📊 **Metrics Collection**
- **Success Rate Tracking**: Monitor workflow success rates over time
- **Performance Metrics**: Track duration, queue times, and retry rates
- **Failure Pattern Detection**: Identify recurring issues and trends
- **Historical Data**: 30-day rolling metrics with CSV export

### 🚨 **Intelligent Alerting**
- **Automatic Alerts**: Triggered when failure rate >20% or success rate <85%
- **Multi-channel Notifications**: Slack, email, and GitHub issues
- **Alert Thresholds**: Configurable warning and critical levels
- **Daily Summaries**: Optional daily health reports

### 📈 **Interactive Dashboard**
- **Visual Charts**: Plotly-powered interactive graphs
- **Real-time Updates**: Auto-refreshed every 6 hours
- **GitHub Pages**: Accessible dashboard at `https://[username].github.io/[repo]/dashboard.html`
- **Mobile Responsive**: Works on all devices

### 🏷️ **Status Badges**
- **Success Rate Badge**: Auto-updating README badge
- **Color-coded**: Green (>95%), Yellow (80-95%), Red (<80%)
- **Historical Tracking**: Trend visualization

## 🚀 Quick Setup

### 1. Enable GitHub Pages

1. Go to **Settings** → **Pages**
2. Set **Source** to "GitHub Actions"
3. The dashboard will be available at: `https://[username].github.io/[repo]/dashboard.html`

### 2. Configure Notifications (Optional)

Add these secrets in **Settings** → **Secrets and variables** → **Actions**:

#### Slack Notifications
```bash
SLACK_WEBHOOK_URL=https://hooks.slack.com/services/YOUR/SLACK/WEBHOOK
```

#### Email Notifications
```bash
EMAIL_TO=alerts@yourcompany.com
EMAIL_USERNAME=smtp-user@gmail.com
EMAIL_PASSWORD=your-app-password
SMTP_SERVER=smtp.gmail.com        # Optional, defaults to Gmail
SMTP_PORT=587                     # Optional, defaults to 587
EMAIL_FROM=noreply@yourcompany.com # Optional, uses EMAIL_USERNAME
```

### 3. Customize Thresholds (Optional)

Edit `.github/workflows/actions-monitoring.yml` to adjust alert thresholds:

```yaml
# Current defaults:
failure_rate_threshold: 0.2    # 20%
retry_rate_threshold: 0.1      # 10%
duration_threshold: 1800       # 30 minutes
```

## 📋 Workflow Schedule

| Workflow | Schedule | Purpose |
|----------|----------|---------|
| **Monitoring** | Every 6 hours | Collect metrics, check thresholds |
| **Dashboard** | Daily at 00:00 UTC | Update visual dashboard |
| **Notifications** | Daily at 09:00 UTC | Send daily summary |

## 🔍 Manual Triggers

All workflows can be triggered manually:

1. Go to **Actions** tab
2. Select the workflow
3. Click **Run workflow**
4. Optionally adjust parameters

## 📊 Dashboard Sections

### Overview Cards
- **Success Rate**: 30-day rolling average
- **Total Runs**: Count of workflow executions
- **Average Duration**: Mean execution time
- **Active Workflows**: Number of distinct workflows

### Charts
1. **Success Rate Trend**: Daily success rate over 30 days
2. **Workflow Performance**: Duration and success rate by workflow
3. **Duration Distribution**: Box plots showing execution time variance
4. **Runs Over Time**: Daily execution counts by workflow

## 🚨 Alert Levels

### 🟢 Normal (No alerts)
- Success rate ≥85%
- Failure rate <15%
- Normal operation

### 🟡 Warning
- Success rate 70-85%
- Failure rate 15-30%
- Notifications sent

### 🔴 Critical
- Success rate <70%
- Failure rate >30%
- Immediate alerts + email

## 📁 Data Storage

### Metrics History
- **Location**: `.github/metrics/`
- **Format**: JSON files with timestamps
- **Retention**: 30 days (120 files)
- **CSV Export**: `trends.csv` for spreadsheet analysis

### Artifacts
- **Metrics Reports**: Downloadable JSON files
- **Retention**: 30 days
- **Size**: Typically <1MB per report

## 🔧 Troubleshooting

### No Metrics Collected
1. Check if workflows are enabled in **Settings** → **Actions**
2. Verify repository has recent workflow runs
3. Ensure `GITHUB_TOKEN` has sufficient permissions

### Dashboard Not Loading
1. Confirm GitHub Pages is enabled
2. Check if dashboard workflow completed successfully
3. Wait up to 10 minutes for Pages deployment

### Notifications Not Sent
1. Verify webhook URLs in secrets
2. Check notification workflow logs
3. Confirm alert thresholds are met

### Missing Charts
1. Ensure Python dependencies installed correctly
2. Check for API rate limiting
3. Verify data processing step completed

## 🎯 Best Practices

### 1. Monitoring Hygiene
- **Review alerts promptly**: Don't ignore warning signs
- **Investigate patterns**: Look for recurring failure causes
- **Update thresholds**: Adjust based on your team's standards

### 2. Dashboard Usage
- **Bookmark the dashboard**: Quick access for team
- **Share with stakeholders**: Transparency in CI/CD health
- **Monitor trends**: Look for gradual degradation

### 3. Notification Management
- **Configure appropriate channels**: Avoid alert fatigue
- **Set up escalation**: Critical alerts to on-call
- **Regular reviews**: Weekly review of notification effectiveness

## 🔗 Integration Examples

### Badge in README
```markdown
[![Actions Dashboard](https://img.shields.io/badge/Actions-Dashboard-blue?style=flat&logo=github)](https://[username].github.io/[repo]/dashboard.html)
```

### Slack Bot Integration
```javascript
// Example webhook handler for custom Slack bot
app.post('/github-actions-alert', (req, res) => {
  const { success_rate, total_runs } = req.body;
  
  if (success_rate < 85) {
    slack.chat.postMessage({
      channel: '#alerts',
      text: `🚨 CI/CD Health Alert: ${success_rate}% success rate`
    });
  }
});
```

### Prometheus Metrics Export
```yaml
# Add to monitoring workflow for Prometheus integration
- name: Export to Prometheus
  run: |
    echo "github_actions_success_rate ${{ success_rate }}" | curl -X POST \
      --data-binary @- http://pushgateway:9091/metrics/job/github-actions
```

## 📈 Advanced Configuration

### Custom Metrics
Extend the monitoring workflow to track additional metrics:

```python
# Add to metrics collection script
custom_metrics = {
    "security_scans": count_security_workflows(runs),
    "deploy_frequency": count_deployments(runs),
    "lead_time": calculate_lead_time(runs)
}
```

### Multi-Repository Monitoring
Create an organization-level workflow that aggregates metrics across repos:

```yaml
strategy:
  matrix:
    repo: [repo1, repo2, repo3]
steps:
  - name: Collect metrics for ${{ matrix.repo }}
    # ... collection logic
```

## 🆘 Support

For issues with the monitoring system:

1. **Check workflow logs**: Actions tab → Workflow run → Job details
2. **Review configuration**: Verify secrets and settings
3. **Create issue**: Use the repository issue tracker
4. **Community help**: GitHub Discussions for questions

## 📚 References

- [GitHub Actions Documentation](https://docs.github.com/en/actions)
- [GitHub REST API](https://docs.github.com/en/rest/actions)
- [Plotly Documentation](https://plotly.com/python/)
- [Slack Webhook Guide](https://api.slack.com/messaging/webhooks)