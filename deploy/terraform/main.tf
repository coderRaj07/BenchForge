# Reviewed BenchForge infrastructure. Apply this module only from a trusted
# checkout. Do not apply Terraform that arrived in a pull request.

terraform {
  required_version = ">= 1.9.0"
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.100"
    }
  }
}

provider "aws" {
  region = var.region
}

variable "region" {
  type    = string
  default = "us-east-1"
}

variable "name" {
  type    = string
  default = "benchforge"
}

resource "aws_vpc" "main" {
  cidr_block           = "10.40.0.0/16"
  enable_dns_support   = true
  enable_dns_hostnames = true
  tags = {
    Name                = var.name
    benchforge_managed  = "true"
  }
}

resource "aws_subnet" "a" {
  vpc_id            = aws_vpc.main.id
  cidr_block        = "10.40.1.0/24"
  availability_zone = "${var.region}a"
  tags = {
    Name               = "${var.name}-a"
    benchforge_managed = "true"
  }
}

resource "aws_s3_bucket" "artifacts" {
  bucket_prefix = "${var.name}-artifacts-"
  tags = {
    benchforge_managed = "true"
  }
}

resource "aws_sqs_queue" "runs" {
  name                       = "${var.name}-runs"
  visibility_timeout_seconds = 900
  message_retention_seconds  = 86400
  tags = {
    benchforge_managed = "true"
  }
}

resource "aws_ecr_repository" "images" {
  name = var.name
  image_tag_mutability = "IMMUTABLE"
  image_scanning_configuration {
    scan_on_push = true
  }
}

resource "aws_iam_role" "worker" {
  name = "${var.name}-worker"
  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect    = "Allow"
      Principal = { Service = "ec2.amazonaws.com" }
      Action    = "sts:AssumeRole"
    }]
  })
}

resource "aws_iam_role_policy" "worker" {
  name = "${var.name}-worker"
  role = aws_iam_role.worker.id
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect   = "Allow"
        Action   = ["s3:GetObject", "s3:PutObject"]
        Resource = "${aws_s3_bucket.artifacts.arn}/*"
      },
      {
        Effect   = "Allow"
        Action   = ["sqs:ReceiveMessage", "sqs:DeleteMessage", "sqs:SendMessage"]
        Resource = aws_sqs_queue.runs.arn
      }
    ]
  })
}

resource "aws_secretsmanager_secret" "api" {
  name_prefix = "${var.name}-api-"
  tags = {
    benchforge_managed = "true"
  }
}

output "artifact_bucket" {
  value = aws_s3_bucket.artifacts.bucket
}

output "run_queue_url" {
  value = aws_sqs_queue.runs.url
}

output "repository_url" {
  value = aws_ecr_repository.images.repository_url
}
