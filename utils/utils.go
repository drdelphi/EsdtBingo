package utils

import (
	"crypto/hmac"
	"crypto/sha512"
	"encoding/binary"
	"fmt"
	"io/ioutil"
	"math"
	"math/big"
	"net/http"
	"strings"

	"github.com/DrDelphi/EsdtBingoBot/data"
	"github.com/ElrondNetwork/elrond-go-crypto/signing"
	"github.com/ElrondNetwork/elrond-go-crypto/signing/ed25519"
	"github.com/btcsuite/btcutil/bech32"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/tyler-smith/go-bip39"
)

const hardened = uint32(0x80000000)

type EsdtTx struct {
	Token  string
	Value  string
	Params []string
}

type bip32Path []uint32

type bip32 struct {
	Key       []byte
	ChainCode []byte
}

var path = bip32Path{
	44 + hardened,
	508 + hardened,
	hardened,
	hardened,
	hardened,
}

func GetHTTP(address string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, address, nil)
	if err != nil {
		return nil, err
	}
	client := http.DefaultClient
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return body, nil
}

func FormatTgUser(user *tgbotapi.User) string {
	name := fmt.Sprintf("%s %s [%v]", user.FirstName, user.LastName, user.ID)
	name = strings.TrimSpace(name)
	name = strings.Replace(name, "  ", " ", 1)
	if user.UserName != "" {
		name = fmt.Sprintf("@%s (%s)", user.UserName, name)
	}

	return name
}

func FormatDbTgUser(user *data.Telegram) string {
	if user.UserName != "" {
		return "@" + user.UserName
	}

	name := fmt.Sprintf("%s %s", user.FirstName, user.LastName)
	name = strings.TrimSpace(name)
	name = strings.Replace(name, "  ", " ", 1)
	name = fmt.Sprintf("[%s](tg://user?id=%v)", name, user.ID)

	return name
}

func GetPrivateKeyFromSeed(index int64) []byte {
	seed := bip39.NewSeed(Seedphrase, "")
	path[3] = hardened + uint32(index>>32)
	path[4] = hardened + uint32(index&0xFFFFFFFF)
	keyData := derivePrivateKey(seed, path)

	return keyData.Key
}

func GetAddressFromPrivateKey(privBytes []byte) string {
	_suite := ed25519.NewEd25519()
	keyGen := signing.NewKeyGenerator(_suite)
	txSignPrivKey, _ := keyGen.PrivateKeyFromByteArray(privBytes)
	pubKey := txSignPrivKey.GeneratePublic()
	pubBytes, _ := pubKey.ToByteArray()
	b, _ := bech32.ConvertBits(pubBytes, 8, 5, true)
	s, _ := bech32.Encode("erd", b)

	return s
}

func derivePrivateKey(seed []byte, path bip32Path) *bip32 {
	b := &bip32{}
	digest := hmac.New(sha512.New, []byte("ed25519 seed"))
	digest.Write(seed)
	intermediary := digest.Sum(nil)
	b.Key = intermediary[:32]
	b.ChainCode = intermediary[32:]
	for _, childIdx := range path {
		data := make([]byte, 1+32+4)
		data[0] = 0x00
		copy(data[1:1+32], b.Key)
		binary.BigEndian.PutUint32(data[1+32:1+32+4], childIdx)
		digest = hmac.New(sha512.New, b.ChainCode)
		digest.Write(data)
		intermediary = digest.Sum(nil)
		b.Key = intermediary[:32]
		b.ChainCode = intermediary[32:]
	}
	return b
}

func NicePrice(f float64, decimals int) string {
	s := fmt.Sprintf("%v", uint64(f))
	for idx := len(s) - 3; idx > 0; idx -= 3 {
		s = s[:idx] + "," + s[idx:]
	}
	if decimals > 0 {
		s += "."
	}
	for i := 0; i < decimals; i++ {
		f -= math.Trunc(f)
		f *= 10
		s += fmt.Sprintf("%v", uint64(f))
	}

	if decimals == -1 { // auto
		if math.Ceil(f) == f {
			return s
		}
		s += "."
		nnd := 0
		nndFound := false
		for i := 0; i < 18; i++ {
			f -= math.Trunc(f)
			f *= 10
			d := uint64(f)
			s += fmt.Sprintf("%v", d)
			if d != 0 && !nndFound {
				nndFound = true
			}
			if nndFound {
				nnd++
				if nnd >= 4 {
					for strings.HasSuffix(s, "0") {
						s = strings.TrimSuffix(s, "0")
					}
					s = strings.TrimSuffix(s, ".")
					break
				}
			}
		}
	}

	return s
}

func ShortenAddress(address string) string {
	l := len(address)
	if l < 14 {
		return ""
	}

	return address[:8] + "..." + address[l-6:]
}

func BytesToTicket(b []byte) *data.Ticket {
	if len(b) < 4 {
		return nil
	}
	len1 := big.NewInt(0).SetBytes(b[:4]).Uint64()
	if len(b) < 4+int(len1) {
		return nil
	}
	b = b[4:]
	b1 := b[:len1]
	b = b[len1:]
	if len(b) < 4 {
		return nil
	}
	len2 := big.NewInt(0).SetBytes(b[:4]).Uint64()
	if len(b) < 4+int(len2) {
		return nil
	}
	b = b[4:]
	b2 := b[:len2]
	b = b[len2:]
	if len(b) < 4 {
		return nil
	}
	len3 := big.NewInt(0).SetBytes(b[:4]).Uint64()
	if len(b) < 4+int(len3) {
		return nil
	}
	b = b[4:]
	b3 := b[:len3]

	n := [3][]byte{BytesToNumbers(b1), BytesToNumbers(b2), BytesToNumbers(b3)}
	ticket := data.Ticket{}
	for i := 0; i < 3; i++ {
		for _, b := range n[i] {
			ticket[i][(b-1)/10] = b
		}
	}

	return &ticket
}

func BytesToNumbers(b []byte) []byte {
	numbers := make([]byte, 0)
	number := byte(0)
	for i := len(b) - 1; i >= 0; i-- {
		b := b[i]
		for bit := 0; bit < 8; bit++ {
			if (1<<bit)&b > 0 {
				numbers = append(numbers, number)
			}
			number++
		}
	}

	return numbers
}

func FormatTicket(ticket *data.Ticket) string {
	res := "----------------------------\n"
	for line := 0; line < 3; line++ {
		res += "|"
		for i := 0; i < 9; i++ {
			if ticket[line][i] == 0 {
				res += "  "
			} else {
				res += fmt.Sprintf("%.02v", ticket[line][i])
			}
			res += "|"
		}
		res += "\n----------------------------\n"
	}

	return "`" + res + "`"
}

func FormatTicketStrikethrough(ticket *data.Ticket, extracted []byte) string {
	res := "----------------------------\n"
	for line := 0; line < 3; line++ {
		res += "|"
		found := 0
		for i := 0; i < 9; i++ {
			if ticket[line][i] == 0 {
				res += "  "
			} else {
				strnumber := fmt.Sprintf("%.02v", ticket[line][i])
				for _, x := range extracted {
					if x == ticket[line][i] {
						strnumber = "`~" + strnumber + "~`"
						found++
						break
					}
				}
				res += strnumber
			}
			res += "|"
		}
		if found == 5 {
			res += " ✅"
		}
		res += "\n----------------------------\n"
	}

	return "`" + res + "`"
}
