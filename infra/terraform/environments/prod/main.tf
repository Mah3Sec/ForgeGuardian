terraform {
  required_version = ">= 1.9"
  required_providers {
    aws = { source = "hashicorp/aws"; version = "~> 5.0" }
  }
  backend "s3" {
    bucket         = "forgeguardian-terraform-state"
    key            = "prod/terraform.tfstate"
    region         = "us-east-1"
    encrypt        = true
    dynamodb_table = "forgeguardian-terraform-locks"
  }
}

provider "aws" {
  region = var.aws_region
  default_tags { tags = { Project = "forgeguardian"; Environment = "prod" } }
}

variable "aws_region"   { default = "us-east-1" }
variable "db_password"  { type = string; sensitive = true }
variable "vpc_id"       { type = string }
variable "subnet_ids"   { type = list(string) }

module "eks" {
  source             = "../../modules/eks"
  cluster_name       = "forgeguardian-prod"
  vpc_id             = var.vpc_id
  subnet_ids         = var.subnet_ids
  desired_nodes      = 3
  node_instance_type = "t3.medium"
}

module "rds" {
  source          = "../../modules/rds"
  identifier      = "forgeguardian-prod"
  vpc_id          = var.vpc_id
  subnet_ids      = var.subnet_ids
  db_password     = var.db_password
  instance_class  = "db.t3.small"
  allowed_sg_ids  = []
}

# S3 bucket for SBOMs and attestations (MinIO replacement in prod)
resource "aws_s3_bucket" "artifacts" {
  bucket = "forgeguardian-artifacts-prod"
}

resource "aws_s3_bucket_versioning" "artifacts" {
  bucket = aws_s3_bucket.artifacts.id
  versioning_configuration { status = "Enabled" }
}

resource "aws_s3_bucket_server_side_encryption_configuration" "artifacts" {
  bucket = aws_s3_bucket.artifacts.id
  rule {
    apply_server_side_encryption_by_default { sse_algorithm = "AES256" }
  }
}

resource "aws_s3_bucket_public_access_block" "artifacts" {
  bucket                  = aws_s3_bucket.artifacts.id
  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

output "eks_endpoint"    { value = module.eks.cluster_endpoint }
output "rds_endpoint"    { value = module.rds.endpoint }
output "artifacts_bucket" { value = aws_s3_bucket.artifacts.bucket }
