resource "aws_s3_bucket" "tides" {
  bucket = "tides-archive"
  tags   = { Name = "Tides" }
}
