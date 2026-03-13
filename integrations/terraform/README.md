# KeepSave Terraform Integration

Use KeepSave secrets in your Terraform infrastructure-as-code workflows.

## Setup

1. Set environment variables:
```bash
export TF_VAR_keepsave_api_url="https://your-keepsave-instance.com"
export TF_VAR_keepsave_token="your-jwt-token"
export TF_VAR_keepsave_project_id="your-project-id"
export TF_VAR_keepsave_environment="prod"
```

2. Reference the module in your Terraform configuration:
```hcl
module "keepsave" {
  source = "./integrations/terraform"

  keepsave_api_url     = var.keepsave_api_url
  keepsave_token       = var.keepsave_token
  keepsave_project_id  = var.keepsave_project_id
  keepsave_environment = "prod"
}
```

## Usage

### Access individual secrets
```hcl
resource "aws_db_instance" "main" {
  username = data.external.keepsave_secrets.result["DB_USERNAME"]
  password = data.external.keepsave_secrets.result["DB_PASSWORD"]
}
```

### Generate .env file
The integration automatically generates a `.env.<environment>` file with all secrets.

## Outputs

| Output | Description |
|--------|-------------|
| `secret_keys` | List of secret key names |
| `secrets_count` | Total number of secrets pulled |
