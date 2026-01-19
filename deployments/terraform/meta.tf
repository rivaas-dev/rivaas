provider "aws" {
  region = "eu-west-1"

  default_tags {
    tags = {
      Environment = var.environment
      Owner       = var.prefix
      Service     = var.service
      CreatedBy   = var.created_by
    }
  }

  assume_role {
    role_arn     = var.assume_role_arn
    session_name = var.assume_role_session_name
  }
}

terraform {
  backend "s3" {}

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}
