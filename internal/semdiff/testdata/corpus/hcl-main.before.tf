resource "aws_s3_bucket" "tides" {
  bucket = "tides-data"
  tags = { Name = "Tides", }
}
