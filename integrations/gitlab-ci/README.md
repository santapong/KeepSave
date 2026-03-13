# KeepSave GitLab CI/CD Integration

Pull secrets from KeepSave into your GitLab CI/CD pipelines.

## Setup

1. Add the following CI/CD variables in your GitLab project settings:
   - `KEEPSAVE_API_URL` - Your KeepSave API URL
   - `KEEPSAVE_API_KEY` - Your KeepSave API key (mark as masked)
   - `KEEPSAVE_PROJECT_ID` - Your KeepSave project ID

2. Include the template in your `.gitlab-ci.yml`:

```yaml
include:
  - local: 'integrations/gitlab-ci/keepsave.gitlab-ci.yml'
```

## Usage

### Pull secrets as environment variables

```yaml
deploy:
  extends: .keepsave-pull
  variables:
    KEEPSAVE_ENVIRONMENT: prod
  script:
    - echo "Deploying with secrets from KeepSave..."
    - ./deploy.sh
```

### Pull secrets to .env file

```yaml
build:
  extends: .keepsave-pull-to-file
  variables:
    KEEPSAVE_ENVIRONMENT: uat
    KEEPSAVE_ENV_FILE: .env.uat
  script:
    - docker build -t myapp .
```

### Promote secrets between environments

```yaml
promote-to-prod:
  extends: .keepsave-promote
  variables:
    KEEPSAVE_FROM: uat
    KEEPSAVE_TO: prod
  when: manual
  only:
    - main
```
