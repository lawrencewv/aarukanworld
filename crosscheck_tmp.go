package main
import (
  "fmt"
  "os"
  chatauth "aarukaninternalchat/internal/auth"
  worldauth "aarukanworld/internal/auth"
)
func main() {
  secret := "cross-check-secret"
  chatauth.ConfigurePlay(secret)
  worldauth.Configure(secret)
  wid := chatauth.WorldIDFromRoom("#aarukan")
  tok, err := chatauth.Sign(chatauth.PlayClaims{Nick:"builder", WorldID:wid, Session:"s1", Gen:1, Exp: 9999999999})
  if err != nil { panic(err) }
  claims, err := worldauth.Verify(tok)
  if err != nil { panic(err) }
  fmt.Println("ok", claims.Nick, claims.WorldID, claims.Gen)
}
