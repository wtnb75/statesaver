terraform {
  backend "http" {
    address = "http://localhost:3000/api/example1"
    lock_address = "http://localhost:3000/api/example1"
    unlock_address = "http://localhost:3000/api/example1"
  }
}
