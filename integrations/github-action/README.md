# KeepSave GitHub Action

Pull secrets from KeepSave and inject them into your GitHub Actions workflow.

## Usage

```yaml
- name: Pull secrets from KeepSave
  uses: santapong/keepsave-action@v1
  with:
    api-url: ${{ secrets.KEEPSAVE_API_URL }}
    api-key: ${{ secrets.KEEPSAVE_API_KEY }}
    project-id: 'your-project-id'
    environment: 'prod'
    export-to: 'env'  # env | file | json
```

## Inputs

| Input | Description | Required | Default |
|-------|-------------|----------|---------|
| `api-url` | KeepSave API URL | Yes | - |
| `api-key` | KeepSave API key | Yes | - |
| `project-id` | KeepSave project ID | Yes | - |
| `environment` | Target environment | Yes | `alpha` |
| `export-to` | Export format (env/file/json) | No | `env` |
| `env-file-path` | Path for .env file output | No | `.env` |

## Outputs

| Output | Description |
|--------|-------------|
| `secrets-count` | Number of secrets pulled |

## Examples

### Export as environment variables
```yaml
jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - uses: santapong/keepsave-action@v1
        with:
          api-url: ${{ secrets.KEEPSAVE_API_URL }}
          api-key: ${{ secrets.KEEPSAVE_API_KEY }}
          project-id: 'my-project-id'
          environment: 'prod'
      - run: echo "Database URL is available as $DATABASE_URL"
```

### Export as .env file
```yaml
      - uses: santapong/keepsave-action@v1
        with:
          api-url: ${{ secrets.KEEPSAVE_API_URL }}
          api-key: ${{ secrets.KEEPSAVE_API_KEY }}
          project-id: 'my-project-id'
          environment: 'uat'
          export-to: 'file'
          env-file-path: '.env.uat'
```
