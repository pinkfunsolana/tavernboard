// Command tavern-agent is a small agent client for TavernBoard.
//
// It drives the full quest lifecycle on the Gno.land Onyx testnet by
// shelling out to gnokey, the official Gno key/tx CLI:
//
//	poster: PostQuest (bounty attached, escrowed in the realm)
//	agent:  ClaimQuest
//	agent:  SubmitProof (URL + sha256 of the proof payload)
//	poster: ConfirmComplete (bounty released, reputation +1)
//
// The same binary exposes each step as its own subcommand so a human or
// another agent can drive the board manually:
//
//	tavern-agent board                 # render the quest board
//	tavern-agent post   -from poster -title "..." -desc "..." -bounty 5000000ugnot -deadline-blocks 100000
//	tavern-agent claim  -from agent  -id 1
//	tavern-agent submit -from agent  -id 1 -url https://example.com/proof -proof "proof text"
//	tavern-agent confirm -from poster -id 1
//	tavern-agent demo                  # the whole flow, both roles
//
// Configuration (flags override env):
//
//	-remote        RPC endpoint        (env TAVERN_REMOTE, default https://rpc.onyx.testnets.gno.land:443)
//	-home          gnokey keybase dir  (env TAVERN_HOME, required)
//	-pkgpath       realm path          (env TAVERN_PKGPATH, default the deployed TavernBoard)
//	-password-env  env var holding the keybase password (default TAVERN_KEYBASE_PASSWORD)
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

const (
	defaultPkgPath = "gno.land/r/g1wcgeganvf6zkpyt8j2tx3h7p3f8afn0my8pw0k/tavernboard/v1"
	defaultRemote  = "https://rpc.onyx.testnets.gno.land:443"
	chainID        = "onyx-1"

	posterKey = "tavern-poster"
	agentKey  = "tavern-agent"
	agentAddr = "g1n9u4cn09v7mdzsusu7k75c6yxnscxdkrcas26u"
)

type config struct {
	gnokey      string
	remote      string
	home        string
	pkgPath     string
	passwordEnv string
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func (c *config) password() string {
	pw := os.Getenv(c.passwordEnv)
	if pw == "" {
		fatalf("keybase password env var %s is empty (never hardcode it; load it from your secret store)", c.passwordEnv)
	}
	return pw
}

// run executes gnokey with the given args, feeding the keybase password on
// stdin when sign is true, and returns combined output.
func (c *config) run(sign bool, args ...string) string {
	full := append([]string{}, args...)
	full = append(full, "-remote", c.remote, "-chainid", chainID)
	if sign {
		full = append(full, "-home", c.home, "-insecure-password-stdin")
	}
	cmd := exec.Command(c.gnokey, full...)
	if sign {
		cmd.Stdin = strings.NewReader(c.password() + "\n")
	}
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		fatalf("gnokey %s failed: %v\n%s", strings.Join(full, " "), err, out.String())
	}
	return out.String()
}

var txHashRe = regexp.MustCompile(`TX HASH:\s+(\S+)`)

func (c *config) call(from, fn, send string, args ...string) string {
	base := []string{"maketx", "call", "-pkgpath", c.pkgPath, "-func", fn,
		"-gas-fee", "10000ugnot", "-gas-wanted", "10000000"}
	for _, a := range args {
		base = append(base, "-args", a)
	}
	if send != "" {
		base = append(base, "-send", send)
	}
	base = append(base, from)
	out := c.run(true, base...)
	m := txHashRe.FindStringSubmatch(out)
	if m == nil {
		fatalf("%s: no tx hash in output:\n%s", fn, out)
	}
	return m[1]
}

func (c *config) query(data string) string {
	out := c.run(false, "query", "vm/qeval", "-data", data)
	// qeval output looks like: data: "(\"...\" string)" or similar; return raw.
	return strings.TrimSpace(out)
}

func (c *config) evalString(expr string) string {
	out := c.query(c.pkgPath + "." + expr)
	// Extract the quoted payload from the qeval result if present.
	if i := strings.Index(out, `"`); i >= 0 {
		if j := strings.LastIndex(out, `"`); j > i {
			return out[i+1 : j]
		}
	}
	return out
}

func (c *config) questCount() uint64 {
	out := c.query(c.pkgPath + ".QuestCount()")
	digits := regexp.MustCompile(`\d+`).FindString(out)
	n, err := strconv.ParseUint(digits, 10, 64)
	if err != nil {
		fatalf("cannot parse QuestCount from %q", out)
	}
	return n
}

func (c *config) balance(addr string) string {
	out := c.run(false, "query", "bank/balances/"+addr)
	if m := regexp.MustCompile(`(\d+)ugnot`).FindStringSubmatch(out); m != nil {
		return m[1] + "ugnot"
	}
	return strings.TrimSpace(out)
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "tavern-agent: "+format+"\n", args...)
	os.Exit(1)
}

func main() {
	cfg := &config{}
	flag.StringVar(&cfg.gnokey, "gnokey", envOr("TAVERN_GNOKEY", "gnokey"), "path to the gnokey binary")
	flag.StringVar(&cfg.remote, "remote", envOr("TAVERN_REMOTE", defaultRemote), "RPC endpoint (use a local relay URL if your network needs a proxy)")
	flag.StringVar(&cfg.home, "home", envOr("TAVERN_HOME", ""), "gnokey keybase directory")
	flag.StringVar(&cfg.pkgPath, "pkgpath", envOr("TAVERN_PKGPATH", defaultPkgPath), "TavernBoard realm path")
	flag.StringVar(&cfg.passwordEnv, "password-env", "TAVERN_KEYBASE_PASSWORD", "env var holding the keybase password")
	flag.Parse()

	if flag.NArg() == 0 {
		fatalf("usage: tavern-agent [flags] <board|post|claim|submit|confirm|demo> [args]")
	}
	if cfg.home == "" && flag.Arg(0) != "board" {
		fatalf("-home (gnokey keybase dir) is required for signing commands")
	}

	switch flag.Arg(0) {
	case "board":
		out := cfg.run(false, "query", "vm/qrender", "-data", cfg.pkgPath+":")
		fmt.Println(out)
	case "post":
		fs := flag.NewFlagSet("post", flag.ExitOnError)
		from := fs.String("from", posterKey, "poster key name")
		title := fs.String("title", "", "quest title")
		desc := fs.String("desc", "", "quest description")
		bounty := fs.String("bounty", "5000000ugnot", "bounty to escrow")
		deadline := fs.Int64("deadline-blocks", 100000, "deadline, in blocks from now")
		fs.Parse(flag.Args()[1:])
		before := cfg.questCount()
		hash := cfg.call(*from, "PostQuest", *bounty, *title, *desc, strconv.FormatInt(*deadline, 10))
		fmt.Printf("PostQuest tx: %s\nnew quest id: %d\n", hash, before+1)
	case "claim":
		fs := flag.NewFlagSet("claim", flag.ExitOnError)
		from := fs.String("from", agentKey, "claimer key name")
		id := fs.Uint64("id", 0, "quest id")
		fs.Parse(flag.Args()[1:])
		hash := cfg.call(*from, "ClaimQuest", "", strconv.FormatUint(*id, 10))
		fmt.Printf("ClaimQuest tx: %s\n", hash)
	case "submit":
		fs := flag.NewFlagSet("submit", flag.ExitOnError)
		from := fs.String("from", agentKey, "claimer key name")
		id := fs.Uint64("id", 0, "quest id")
		url := fs.String("url", "", "proof URL")
		proof := fs.String("proof", "", "proof payload text (its sha256 is submitted as the proof hash)")
		fs.Parse(flag.Args()[1:])
		sum := sha256.Sum256([]byte(*proof))
		hash := cfg.call(*from, "SubmitProof", "", strconv.FormatUint(*id, 10), *url, hex.EncodeToString(sum[:]))
		fmt.Printf("SubmitProof tx: %s\nproof sha256: %s\n", hash, hex.EncodeToString(sum[:]))
	case "confirm":
		fs := flag.NewFlagSet("confirm", flag.ExitOnError)
		from := fs.String("from", posterKey, "poster key name")
		id := fs.Uint64("id", 0, "quest id")
		fs.Parse(flag.Args()[1:])
		hash := cfg.call(*from, "ConfirmComplete", "", strconv.FormatUint(*id, 10))
		fmt.Printf("ConfirmComplete tx: %s\n", hash)
	case "demo":
		runDemo(cfg)
	default:
		fatalf("unknown command %q", flag.Arg(0))
	}
}

// runDemo executes the complete quest lifecycle with the project keys and
// prints a summary with every transaction hash.
func runDemo(c *config) {
	fmt.Println("== TavernBoard demo on Onyx testnet ==")
	fmt.Println("realm:", c.pkgPath)
	fmt.Println("agent balance before:", c.balance(agentAddr))

	before := c.questCount()
	id := before + 1
	fmt.Printf("\n[1/4] poster posts quest #%d (bounty 5000000ugnot = 5 GNOT, escrowed)\n", id)
	h := c.call(posterKey, "PostQuest", "5000000ugnot",
		"Translate the tavern menu into Gnomish",
		"Translate the 12-item Gnome Tavern menu into Gnomish and publish the result.",
		"100000")
	fmt.Println("  tx:", h)
	fmt.Println("  board:", c.evalString(fmt.Sprintf("GetQuest(%d)", id)))

	fmt.Printf("\n[2/4] agent claims quest #%d\n", id)
	h = c.call(agentKey, "ClaimQuest", "", strconv.FormatUint(id, 10))
	fmt.Println("  tx:", h)

	fmt.Printf("\n[3/4] agent submits proof for quest #%d\n", id)
	proofText := "Gnomish menu translation, 12 items, delivered by tavern-agent."
	sum := sha256.Sum256([]byte(proofText))
	h = c.call(agentKey, "SubmitProof", "",
		strconv.FormatUint(id, 10),
		"https://example.com/tavern/menu-gnomish",
		hex.EncodeToString(sum[:]))
	fmt.Println("  tx:", h)
	fmt.Println("  proof sha256:", hex.EncodeToString(sum[:]))

	fmt.Printf("\n[4/4] poster confirms completion — bounty released, reputation +1\n")
	h = c.call(posterKey, "ConfirmComplete", "", strconv.FormatUint(id, 10))
	fmt.Println("  tx:", h)

	fmt.Println("\n== result ==")
	fmt.Println("quest:", c.evalString(fmt.Sprintf("GetQuest(%d)", id)))
	fmt.Println("agent reputation:", c.evalString(fmt.Sprintf("ReputationOf(%q)", agentAddr)))
	fmt.Println("agent balance after:", c.balance(agentAddr))
}
