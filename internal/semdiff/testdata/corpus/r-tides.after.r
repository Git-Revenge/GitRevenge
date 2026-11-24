max_hours <- 72
forecast <- function(n, step = 3600) {
  readings <- seq_len(n) * step
  readings
}
