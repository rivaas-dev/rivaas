locals {
  prefix = "${var.prefix}-${var.environment}"
}

resource "aws_iam_role" "this" {
  name = "${var.environment}-admin-api"
  path = "/${var.prefix}/"

  assume_role_policy = jsonencode({
    Version   = "2012-10-17"
    Statement = [
      {
        Effect    = "Allow"
        Action    = ["sts:AssumeRoleWithWebIdentity"]
        Principal = {
          Federated = local.oidc_idp_arn
        },
        Condition = {
          StringEquals = {
            "${local.oidc_idp_name}:aud" = "sts.amazonaws.com"
            "${local.oidc_idp_name}:sub" = ["system:serviceaccount:${var.service_account}"]
          }
        }
      }
    ]
  })
}

resource "aws_iam_role_policy_attachment" "this" {
  role       = aws_iam_role.this.name
  policy_arn = aws_iam_policy.this.arn
}

resource "aws_iam_policy" "this" {
  name = "${var.environment}-admin-api"
  path = "/${var.prefix}/"

  policy = jsonencode({
    Version   = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "events:PutEvents"
        ]
        Resource = [
          var.event_bus_arn
        ]
      }
    ]
  })
}
