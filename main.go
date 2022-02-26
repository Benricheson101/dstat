package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"sync/atomic"
	"time"

	tm "github.com/buger/goterm"
	"github.com/bwmarrin/discordgo"
)

type DStatOutput struct {
	Guilds               int32 `json:"guild_count"`
	UnavailableGuilds    int32 `json:"unavailable_guilds"`
	RecommendedShards    int32 `json:"recommended_shards"`
	ReadyShards          int32 `json:"-"`
	MaxConcurrency       int32 `json:"-"`
	MemberCount          int32 `json:"member_count"`
	LargestGuildSize     int32 `json:"largest_guild_size"`
	GT100k               int32 `json:"gt_100k"`
	GT10k                int32 `json:"gt_10k"`
	GT1k                 int32 `json:"gt_1k"`
	PartnerCount         int32 `json:"partner_count"`
	VerifiedCount        int32 `json:"verified_count"`
	VerifiedPartnerCount int32 `json:"verified_partner_count"`
}

var (
	out DStatOutput

	// CLI flags
	outputJson   bool
	noLiveOutput bool
	token        string
)

const GUILD_READY_TIMEOUT time.Duration = time.Second * 20

func init() {
	flag.BoolVar(&outputJson, "json", false, "output JSON instead of a formatted list. useful for programmatic usage")
	flag.BoolVar(&noLiveOutput, "nolive", false, "disables live output")
	flag.StringVar(&token, "token", os.Getenv("DISCORD_TOKEN"), "the discord token to connect with")

	flag.Parse()

	// always output JSON if output is not a TTY
	if fileInfo, _ := os.Stdout.Stat(); (fileInfo.Mode() & os.ModeCharDevice) == 0 {
		outputJson = true
		noLiveOutput = true
	}
}

func main() {
	gwInfo := GetGatewayAuthed(token)
	out.RecommendedShards = int32(gwInfo.Shards)
	out.MaxConcurrency = int32(gwInfo.SessionStartLimit.MaxConcurrency)

	if !outputJson && !noLiveOutput {
		go func() {
			for {
				updateScreen()
				time.Sleep(time.Millisecond * 500)
			}
		}()
	}

	var wg sync.WaitGroup
	shardsStarted := 0
	for i := 0; i < gwInfo.Shards; i++ {
		shardsStarted++

		wg.Add(1)
		go func(i int) {
			defer wg.Done()

			done := make(chan bool)
			s := createSession(token, i, gwInfo.Shards, done)
			s.Open()
			<-done
		}(i)

		if shardsStarted%gwInfo.SessionStartLimit.MaxConcurrency == 0 && shardsStarted != gwInfo.Shards {
			time.Sleep(time.Second * 5)
		}
	}

	wg.Wait()

	if outputJson {
		j, _ := json.Marshal(&out)
		fmt.Print(string(j))
	} else if noLiveOutput {
		out.FormatToWriter(os.Stdout)
	}
}

// TODO: use a WaitGroup or Context instead of done channel?
func createSession(token string, shardId, shardCount int, done chan<- bool) *discordgo.Session {
	s, _ := discordgo.New("Bot " + token)
	s.StateEnabled = false
	s.Identify.Shard = &[2]int{shardId, shardCount}
	s.Identify.Intents = discordgo.IntentsGuilds

	var shardGuildCount int32 = 0
	var guildsReceived int32 = 0

	s.AddHandler(func(s *discordgo.Session, r *discordgo.Ready) {
		shardGuildCount = int32(len(r.Guilds))
		atomic.AddInt32(&out.Guilds, int32(len(r.Guilds)))
		atomic.AddInt32(&out.ReadyShards, 1)

		var wg sync.WaitGroup
		wg.Add(1)
		go func(shId int) {
			defer wg.Done()
			time.Sleep(GUILD_READY_TIMEOUT)

			g := atomic.LoadInt32(&guildsReceived)
			if shardGuildCount-g > 0 {
				atomic.AddInt32(&out.UnavailableGuilds, shardGuildCount-g)
				s.Close()
				done <- true
			}
		}(shardId)
		wg.Wait()
	})

	s.AddHandler(func(s *discordgo.Session, gc *discordgo.GuildCreate) {
		members := int32(gc.MemberCount)

		shardGc := atomic.AddInt32(&guildsReceived, 1)
		atomic.AddInt32(&out.MemberCount, members)

		if members > 100_000 {
			atomic.AddInt32(&out.GT100k, 1)
		}

		if members > 10_000 {
			atomic.AddInt32(&out.GT10k, 1)
		}

		if members > 1_000 {
			atomic.AddInt32(&out.GT1k, 1)
		}

		l := atomic.LoadInt32(&out.LargestGuildSize)

		if members > l {
			atomic.StoreInt32(&out.LargestGuildSize, members)
		}

		isPartner := strListContains(gc.Features, "PARTNERED")
		isVerified := strListContains(gc.Features, "VERIFIED")

		if isPartner {
			atomic.AddInt32(&out.PartnerCount, 1)
		}

		if isVerified {
			atomic.AddInt32(&out.VerifiedCount, 1)
		}

		if isPartner && isVerified {
			atomic.AddInt32(&out.VerifiedPartnerCount, 1)
		}

		if shardGc == shardGuildCount {
			s.Close()
			done <- true
		}
	})

	return s
}

func strListContains(list []string, str string) bool {
	for _, s := range list {
		if s == str {
			return true
		}
	}

	return false
}

func (ds *DStatOutput) MarshalJSON() ([]byte, error) {
	avgGuildSize := ds.MemberCount / ds.Guilds

	type Alias DStatOutput

	return json.Marshal(&struct {
		*Alias
		AvgMemberCount int32 `json:"average_member_count"`
	}{
		Alias:          (*Alias)(ds),
		AvgMemberCount: avgGuildSize,
	})
}

func (ds *DStatOutput) FormatToWriter(w io.Writer) {
	guilds := atomic.LoadInt32(&out.Guilds)
	members := atomic.LoadInt32(&out.MemberCount)
	fmt.Fprintln(w, "Guilds             =>", guilds)
	fmt.Fprintln(w, "Unavailable Guilds =>", atomic.LoadInt32(&out.UnavailableGuilds))
	fmt.Fprintln(w, "Ready Shards       =>", atomic.LoadInt32(&out.ReadyShards))
	fmt.Fprintln(w, "Recommended Shards =>", atomic.LoadInt32(&out.RecommendedShards))
	fmt.Fprintln(w, "Member Count       =>", members)
	fmt.Fprintln(w, ">= 100,000         =>", atomic.LoadInt32(&out.GT100k))
	fmt.Fprintln(w, ">= 10,000          =>", atomic.LoadInt32(&out.GT10k))
	fmt.Fprintln(w, ">= 1,000           =>", atomic.LoadInt32(&out.GT1k))
	fmt.Fprintln(w, "Partnered          =>", atomic.LoadInt32(&out.PartnerCount))
	fmt.Fprintln(w, "Verified           =>", atomic.LoadInt32(&out.VerifiedCount))
	fmt.Fprintln(w, "P & V              =>", atomic.LoadInt32(&out.VerifiedPartnerCount))
	fmt.Fprintln(w, "Largest            =>", atomic.LoadInt32(&out.LargestGuildSize))

	g := guilds
	if guilds == 0 {
		g = 1
	}
	fmt.Fprintln(w, "Avg Guild Size     =>", members/g)
}

func updateScreen() {
	tm.Clear()
	tm.MoveCursor(1, 1)
	out.FormatToWriter(tm.Screen)
	tm.Flush()
}

type GatewayBot struct {
	Shards            int    `json:"shards"`
	URL               string `json:"url"`
	SessionStartLimit struct {
		MaxConcurrency int `json:"max_concurrency"`
		Remaining      int `json:"Remaining"`
		ResetAfter     int `json:"reset_after"`
		Total          int `json:"total"`
	} `json:"session_start_limit"`
}

func GetGatewayAuthed(token string) *GatewayBot {
	req, _ := http.NewRequest("GET", "https://discord.com/api/v10/gateway/bot", nil)
	req.Header.Set("Authorization", "Bot "+token)

	res, _ := http.DefaultClient.Do(req)
	var body GatewayBot
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		panic(err)
	}

	return &body
}
