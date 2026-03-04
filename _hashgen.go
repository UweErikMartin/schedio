package main
import ("fmt"; "golang.org/x/crypto/bcrypt")
func main() {
  for _, pw := range []string{"changeme","admin123"} {
    h, _ := bcrypt.GenerateFromPassword([]byte(pw), 12)
    fmt.Printf("password=%q\nhash=%s\n\n", pw, h)
  }
}
