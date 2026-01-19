locals {
  # OIDC IdP Name
  oidc_idp_name = "oidc.eks.eu-west-1.amazonaws.com/id/CA9515D2FC9938D76228D437EEE75AAF"
  # OIDC IdP ARN
  oidc_idp_arn  = "arn:aws:iam::220845518108:oidc-provider/${local.oidc_idp_name}"
}

variable "prefix" {
  type        = string
  description = "prefix to resource names"
  default     = "info-company-nl-ci-api"
}

variable "environment" {
  type = string
  validation {
    condition     = var.environment == "acc" || var.environment == "prod"
    error_message = "The environment value must be \"acc\" or \"prod\"."
  }
}

variable "service" {
  type        = string
  description = "The name of the service for which resources were created."
}

variable "created_by" {
  type        = string
  description = "The name of the party that applied the resources on AWS."
}

variable "service_account" {
  type        = string
  description = "The project's service account"
}

variable "assume_role_arn" {
  type        = string
  description = "The ARN of the role to Assume"
}

variable "assume_role_session_name" {
  type        = string
  description = "What to name the session when assuming the role"
}

variable "event_bus_arn" {
  type        = string
  description = "The ARN of the Event Bus"
}
