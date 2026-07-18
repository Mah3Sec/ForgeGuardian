variable "identifier"    { type = string }
variable "vpc_id"        { type = string }
variable "subnet_ids"    { type = list(string) }
variable "db_password"   { type = string; sensitive = true }
variable "instance_class" { type = string; default = "db.t3.micro" }
variable "allowed_sg_ids" { type = list(string); default = [] }

resource "aws_db_subnet_group" "this" {
  name       = "${var.identifier}-subnet-group"
  subnet_ids = var.subnet_ids
}

resource "aws_security_group" "rds" {
  name   = "${var.identifier}-rds-sg"
  vpc_id = var.vpc_id

  ingress {
    from_port       = 5432
    to_port         = 5432
    protocol        = "tcp"
    security_groups = var.allowed_sg_ids
  }
  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }
}

resource "aws_db_instance" "this" {
  identifier              = var.identifier
  engine                  = "postgres"
  engine_version          = "16"
  instance_class          = var.instance_class
  allocated_storage       = 20
  max_allocated_storage   = 100
  storage_encrypted       = true
  username                = "forgeguardian"
  password                = var.db_password
  db_name                 = "forgeguardian"
  db_subnet_group_name    = aws_db_subnet_group.this.name
  vpc_security_group_ids  = [aws_security_group.rds.id]
  backup_retention_period = 7
  deletion_protection     = true
  skip_final_snapshot     = false
  final_snapshot_identifier = "${var.identifier}-final"
}

output "endpoint" { value = aws_db_instance.this.endpoint }
output "port"     { value = aws_db_instance.this.port }
