package main

import (
	"bytes"
	"encoding/binary"
	"flag"
	"fmt"
	"io"
	"math/rand"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/bwmarrin/discordgo"
	"github.com/dustin/go-humanize"
	redis "gopkg.in/redis.v3"
)

var (
	// discordgo session
	discord *discordgo.Session

	// Redis client connection (used for stats)
	rcli *redis.Client

	// Map of Guild id's to *Play channels, used for queuing and rate-limiting guilds
	queues map[string]chan *Play = make(map[string]chan *Play)

	// Sound encoding settings
	BITRATE        = 128
	MAX_QUEUE_SIZE = 6

	// Owner
	OWNER string

	// Shard (or -1)
	SHARDS []string = make([]string, 0)
)

// Play represents an individual use of the !airhorn command
type Play struct {
	GuildID   string
	ChannelID string
	UserID    string
	Sound     *Sound

	// The next play to occur after this, only used for chaining sounds like anotha
	Next *Play

	// If true, this was a forced play using a specific airhorn sound name
	Forced bool
}

type SoundCollection struct {
	Prefix    string
	Commands  []string
	Sounds    []*Sound
	ChainWith *SoundCollection

	soundRange int
}

// Sound represents a sound clip
type Sound struct {
	Name string

	// Weight adjust how likely it is this song will play, higher = more likely
	Weight int

	// Delay (in milliseconds) for the bot to wait before sending the disconnect request
	PartDelay int

	// Buffer to store encoded PCM packets
	buffer [][]byte
}

// Array of all the sounds we have
var AIRHORN *SoundCollection = &SoundCollection{
	Prefix: "airhorn",
	Commands: []string{
		"!airhorn",
	},
	Sounds: []*Sound{
		createSound("default", 1000, 250),
		createSound("reverb", 800, 250),
		createSound("spam", 800, 0),
		createSound("tripletap", 800, 250),
		createSound("fourtap", 800, 250),
		createSound("distant", 500, 250),
		createSound("echo", 500, 250),
		createSound("clownfull", 250, 250),
		createSound("clownshort", 250, 250),
		createSound("clownspam", 250, 0),
		createSound("highfartlong", 200, 250),
		createSound("highfartshort", 200, 250),
		createSound("midshort", 100, 250),
		createSound("truck", 10, 250),
	},
}

var KHALED *SoundCollection = &SoundCollection{
	Prefix:    "another",
	ChainWith: AIRHORN,
	Commands: []string{
		"!anotha",
		"!anothaone",
	},
	Sounds: []*Sound{
		createSound("one", 1, 250),
		createSound("one_classic", 1, 250),
		createSound("one_echo", 1, 250),
	},
}

var CENA *SoundCollection = &SoundCollection{
	Prefix: "jc",
	Commands: []string{
		"!johncena",
		"!cena",
	},
	Sounds: []*Sound{
		createSound("airhorn", 1, 250),
		createSound("echo", 1, 250),
		createSound("full", 1, 250),
		createSound("jc", 1, 250),
		createSound("nameis", 1, 250),
		createSound("spam", 1, 250),
	},
}

var ETHAN *SoundCollection = &SoundCollection{
	Prefix: "ethan",
	Commands: []string{
		"!ethan",
		"!eb",
		"!ethanbradberry",
		"!h3h3",
	},
	Sounds: []*Sound{
		createSound("areyou_classic", 100, 250),
		createSound("areyou_condensed", 100, 250),
		createSound("areyou_crazy", 100, 250),
		createSound("areyou_ethan", 100, 250),
		createSound("classic", 100, 250),
		createSound("echo", 100, 250),
		createSound("high", 100, 250),
		createSound("slowandlow", 100, 250),
		createSound("cuts", 30, 250),
		createSound("beat", 30, 250),
		createSound("sodiepop", 1, 250),
	},
}

var COW *SoundCollection = &SoundCollection{
	Prefix: "cow",
	Commands: []string{
		"!stan",
		"!stanislav",
	},
	Sounds: []*Sound{
		createSound("herd", 10, 250),
		createSound("moo", 10, 250),
		createSound("x3", 1, 250),
	},
}
var LOWETHAN *SoundCollection = &SoundCollection{
	Prefix: "custom",
	Commands: []string{
		"!lowethan",
	},
	Sounds: []*Sound{
		createSound("lowethan", 1, 250),
	},
}

var HIGHETHAN *SoundCollection = &SoundCollection{
	Prefix: "custom",
	Commands: []string{
		"!highethan",
	},
	Sounds: []*Sound{
		createSound("highethan", 1, 250),
	},
}

var MULTIETHAN *SoundCollection = &SoundCollection{
	Prefix: "custom",
	Commands: []string{
		"!multiethan",
	},
	Sounds: []*Sound{
		createSound("multiethan", 1, 250),
	},
}

var CENANEW *SoundCollection = &SoundCollection{
	Prefix: "custom",
	Commands: []string{
		"!truecena",
	},
	Sounds: []*Sound{
		createSound("cena", 1, 250),
	},
}

var CAPTAIN *SoundCollection = &SoundCollection{
	Prefix: "custom",
	Commands: []string{
		"!captain",
	},
	Sounds: []*Sound{
		createSound("captain", 1, 250),
	},
}

var JEFF *SoundCollection = &SoundCollection{
	Prefix: "custom",
	Commands: []string{
		"!jeff",
	},
	Sounds: []*Sound{
		createSound("jeff", 1, 250),
	},
}

var ETHANCENA *SoundCollection = &SoundCollection{
	Prefix: "custom",
	Commands: []string{
		"!ethancena",
	},
	Sounds: []*Sound{
		createSound("ethancena", 1, 250),
	},
}

var FUSRODAH *SoundCollection = &SoundCollection{
	Prefix: "custom",
	Commands: []string{
		"!fusrodah",
	},
	Sounds: []*Sound{
		createSound("fusrodah", 1, 250),
	},
}

var BASE *SoundCollection = &SoundCollection{
	Prefix: "custom",
	Commands: []string{
		"!base",
		"!bass",
	},
	Sounds: []*Sound{
		createSound("base", 1, 250),
	},
}

var LEEROY *SoundCollection = &SoundCollection{
	Prefix: "custom",
	Commands: []string{
		"!leeroy",
	},
	Sounds: []*Sound{
		createSound("leeroy", 1, 250),
	},
}

var LEEROYFULL *SoundCollection = &SoundCollection{
	Prefix: "custom",
	Commands: []string{
		"!leeroyfull",
	},
	Sounds: []*Sound{
		createSound("leeroyfull", 1, 250),
	},
}

var AKBAR *SoundCollection = &SoundCollection{
	Prefix: "custom",
	Commands: []string{
		"!akbar",
		"!allahuakbar",
	},
	Sounds: []*Sound{
		createSound("akbar", 1, 250),
	},
}
var MAD *SoundCollection = &SoundCollection{
	Prefix: "custom",
	Commands: []string{
		"!mad",
	},
	Sounds: []*Sound{
		createSound("mad", 1, 250),
	},
}

var CODEC *SoundCollection = &SoundCollection{
	Prefix: "custom",
	Commands: []string{
		"!codec",
	},
	Sounds: []*Sound{
		createSound("codec", 1, 250),
	},
}

var NTHOU *SoundCollection = &SoundCollection{
	Prefix: "custom",
	Commands: []string{
		"!9000",
	},
	Sounds: []*Sound{
		createSound("9000", 1, 250),
	},
}

var NTHOUFULL *SoundCollection = &SoundCollection{
	Prefix: "custom",
	Commands: []string{
		"!9000full",
	},
	Sounds: []*Sound{
		createSound("9000extended", 1, 250),
	},
}

var SURPRISE *SoundCollection = &SoundCollection{
	Prefix: "surprise",
	Commands: []string{
		"!surprise",
	},
	Sounds: []*Sound{
		createSound("disguise", 1, 250),
		createSound("fries", 1, 250),
		createSound("pies", 1, 250),
		createSound("rise", 1, 250),
		createSound("size", 1, 250),
		createSound("supplies", 1, 250),
		createSound("surprise", 1, 250),
	},
}

var WOMBO *SoundCollection = &SoundCollection{
	Prefix: "custom",
	Commands: []string{
		"!wombo",
		"!wombocombo",
	},
	Sounds: []*Sound{
		createSound("wombo", 1, 250),
	},
}

var LOSE *SoundCollection = &SoundCollection{
	Prefix: "custom",
	Commands: []string{
		"!lose",
	},
	Sounds: []*Sound{
		createSound("lose", 1, 250),
	},
}

var SPOOKY *SoundCollection = &SoundCollection{
	Prefix: "custom",
	Commands: []string{
		"!spooky",
		"!spoopy",
	},
	Sounds: []*Sound{
		createSound("spooky", 1, 250),
	},
}

var ERRYDAY *SoundCollection = &SoundCollection{
	Prefix: "custom",
	Commands: []string{
		"!erryday",
	},
	Sounds: []*Sound{
		createSound("erryday", 1, 250),
	},
}
var EMINEM *SoundCollection = &SoundCollection{
	Prefix: "custom",
	Commands: []string{
		"!eminem",
	},
	Sounds: []*Sound{
		createSound("eminem", 1, 250),
	},
}
var DARKNESS *SoundCollection = &SoundCollection{
	Prefix: "custom",
	Commands: []string{
		"!darkness",
	},
	Sounds: []*Sound{
		createSound("darkness", 1, 250),
	},
}

var JOHNCENA *SoundCollection = &SoundCollection{
	Prefix: "custom",
	Commands: []string{
		"!morecena",
	},
	Sounds: []*Sound{
		createSound("johncena", 1, 250),
	},
}
var WORLD *SoundCollection = &SoundCollection{
	Prefix: "custom",
	Commands: []string{
		"!zawarudo",
		"!world",
	},
	Sounds: []*Sound{
		createSound("zawarudo", 1, 250),
	},
}
var YOULOSE *SoundCollection = &SoundCollection{
	Prefix: "custom",
	Commands: []string{
		"!youlose",
	},
	Sounds: []*Sound{
		createSound("youlose", 1, 250),
	},
}
var ALLIGATOR *SoundCollection = &SoundCollection{
	Prefix: "custom",
	Commands: []string{
		"!alligator",
	},
	Sounds: []*Sound{
		createSound("alligator", 1, 250),
	},
}
var NOTIME *SoundCollection = &SoundCollection{
	Prefix: "custom",
	Commands: []string{
		"!notime",
	},
	Sounds: []*Sound{
		createSound("notime", 1, 250),
	},
}
var PICCOLO *SoundCollection = &SoundCollection{
        Prefix: "Piccolo",
        Commands: []string{
                "!piccolo",
        },
        Sounds: []*Sound{
                createSound("gohan", 1, 250),
		createSound("inert", 1, 250),
		createSound("pants", 1, 250),
		createSound("shit", 1, 250),
		createSound("yes", 1, 250),
		createSound("dende", 1, 250),
		createSound("difference", 1, 250),
		createSound("stress", 1, 250),
		createSound("man", 1, 250),
        },
}
var SHIA *SoundCollection = &SoundCollection{
	Prefix: "shia",
	Commands: []string{
		"!shia",
	},
	Sounds: []*Sound{
		createSound("demotivate", 1, 250),
		createSound("doit1", 5, 250),
		createSound("doit2", 5, 250),
		createSound("doit3", 5, 250),
		createSound("doit4", 5, 250),
		createSound("dreams", 5, 250),
		createSound("nothing", 5, 250),
		createSound("quit", 1, 250),
	},
}

var MARIO *SoundCollection = &SoundCollection{
	Prefix: "mario",
	Commands: []string{
		"!mario",
	},
	Sounds: []*Sound{
		createSound("cena", 1, 250),
		createSound("eb", 1, 250),
		createSound("jeff", 1, 250),
	},
}

var FFFF *SoundCollection = &SoundCollection{
	Prefix: "custom",
	Commands: []string{
		"!ffff",
	},
	Sounds: []*Sound{
		createSound("ffff", 1, 250),
	},
}

var NEUTRAL *SoundCollection = &SoundCollection{
	Prefix: "custom",
	Commands: []string{
		"!neutral",
	},
	Sounds: []*Sound{
		createSound("neutral", 1, 250),
	},
}

var UMAD *SoundCollection = &SoundCollection{
	Prefix: "custom",
	Commands: []string{
		"!umad",
	},
	Sounds: []*Sound{
		createSound("umad", 1, 250),
	},
}
var RICK *SoundCollection = &SoundCollection{
	Prefix: "rick",
	Commands: []string{
		"!rick",
	},
	Sounds: []*Sound{
		createSound("whatugot", 1, 250),
		createSound("aids", 1, 250),
		createSound("maballs", 1, 250),
		createSound("wuba", 1, 250),
	},
}
var FEELSBAD *SoundCollection = &SoundCollection{
        Prefix: "custom",
        Commands: []string{
                "!feelsbad",
        },
        Sounds: []*Sound{
                createSound("feelsbad", 1, 250),
        },
}
var TWISTED *SoundCollection = &SoundCollection{
        Prefix: "custom",
        Commands: []string{
                "!twisted",
		"!twistedsister",
        },
        Sounds: []*Sound{
                createSound("twisted", 1, 250),
        },
}
var NIC *SoundCollection = &SoundCollection{
	Prefix: "nic",
	Commands: []string{
		"!nic",
	},
Sounds: []*Sound{
		createSound("rodger", 1, 250),
		createSound("jaguar", 1, 250),
		createSound("ahole", 1, 250),
		createSound("silence", 1, 250),
		createSound("shark", 1, 250),
		createSound("hahaa", 1, 250),
		createSound("aight", 1, 250),
		createSound("mouth", 1, 250),
		createSound("cool", 1, 250),
		createSound("asskick", 1, 250),
		createSound("bunny", 1, 250),
		createSound("jungle", 1, 250),
		createSound("drugyou", 1, 250),
		createSound("haggis", 1, 250),
		createSound("littletired", 1, 250),
		createSound("executioner", 1, 250),
		createSound("zeus", 1, 250),
		createSound("donotwant", 1, 250),
		createSound("expression", 1, 250),
		createSound("prick", 1, 250),
		createSound("captainjack", 1, 250),
		createSound("squarefootage", 1, 250),
		createSound("killme", 1, 250),
		createSound("pear", 1, 250),
		createSound("retard", 1, 250),
		createSound("notcool", 1, 250),
		createSound("sidewalk", 1, 250),
		createSound("pissedblood", 1, 250),
		createSound("hand", 1, 250),
		createSound("allgone", 1, 250),
		createSound("french", 1, 250),
		createSound("burned", 1, 250),
		createSound("jokerswild", 1, 250),
		createSound("abc", 1, 250),
		createSound("boner", 1, 250),
		createSound("passingthrough", 1, 250),
		createSound("knowthatnow", 1, 250),
		createSound("thereyouare", 1, 250),
		createSound("gettingthrough", 1, 250),
		createSound("pressuring", 1, 250),
		createSound("weewee", 1, 250),
		createSound("peach", 1, 250),
		createSound("king", 1, 250),
		createSound("stinky", 1, 250),
		createSound("filthy", 1, 250),
		createSound("switch", 1, 250),
		createSound("honey", 1, 250),
		createSound("shameonyou", 1, 250),
		createSound("misfile", 1, 250),
		createSound("hallelujah", 1, 250),
		createSound("goesaway", 1, 250),
		createSound("imavampire", 1, 250),
		createSound("bees", 1, 250),
		createSound("kickyourass", 1, 250),
		createSound("bullshit", 1, 250),
		createSound("fuckmexico", 1, 250),
		createSound("fuckyou", 1, 250),
		createSound("hiya", 1, 250),
		createSound("sp1", 1, 250),
	},
}

var ERANU *SoundCollection = &SoundCollection{
	Prefix: "custom",
	Commands: []string{
		"!eranu",
	},
	Sounds: []*Sound{
		createSound("eranu", 1, 250),
	},
}

var FMJ *SoundCollection = &SoundCollection{
	Prefix: "fmj",
	Commands: []string{
		"!fmj",
	},
	Sounds: []*Sound{
		createSound("doyousuck", 1, 250),
		createSound("shitdown", 1, 250),
		createSound("sirwhat", 1, 250),
		createSound("skullfuck", 1, 250),
		createSound("tiffanycufflinks", 1, 250),
		createSound("amphibianshit", 1, 250),
	},
}

var IDEAGAY *SoundCollection = &SoundCollection{
	Prefix: "custom",
	Commands: []string{
		"!totallygay",
	},
	Sounds: []*Sound{
		createSound("ideagay", 1, 250),
	},
}

var HAHGAY *SoundCollection = &SoundCollection{
	Prefix: "custom",
	Commands: []string{
		"!hahgay",
	},
	Sounds: []*Sound{
		createSound("hahgay", 1, 250),
	},
}

var SMOKE *SoundCollection = &SoundCollection{
	Prefix: "custom",
	Commands: []string{
		"!smoke",
	},
	Sounds: []*Sound{
		createSound("smoke", 1, 250),
	},
}

var OVERWATCH *SoundCollection = &SoundCollection{
	Prefix: "overwatch",
	Commands: []string{
		"!ow",
		"!overwatch",
	},
	Sounds: []*Sound{
		createSound("dejavu", 1, 250),
	},
}

var JABRONI *SoundCollection = &SoundCollection{
	Prefix: "custom",
	Commands: []string{
		"!jabroni",
	},
	Sounds: []*Sound{
		createSound("jabroni", 1, 250),
	},
}

var MUDA *SoundCollection = &SoundCollection{
	Prefix: "custom",
	Commands: []string{
		"!muda",
	},
	Sounds: []*Sound{
		createSound("muda", 1, 250),
	},
}

var FEVER *SoundCollection = &SoundCollection{
	Prefix: "custom",
	Commands: []string{
		"!fever",
	},
	Sounds: []*Sound{
		createSound("fever", 1, 250),
	},
}

var COMEWITHME *SoundCollection = &SoundCollection{
	Prefix: "custom",
	Commands: []string{
		"!comewithme",
	},
	Sounds: []*Sound{
		createSound("comewithme", 1, 250),
	},
}

var BRERBRER *SoundCollection = &SoundCollection{
	Prefix: "custom",
	Commands: []string{
		"!brerbrer",
		"!fag",
	},
	Sounds: []*Sound{
		createSound("brerbrer", 1, 250),
	},
}

var COOKIE *SoundCollection = &SoundCollection{
	Prefix: "custom",
	Commands: []string{
		"!cookie",
	},
	Sounds: []*Sound{
		createSound("cookie", 1, 250),
	},
}

var PAYLOAD *SoundCollection = &SoundCollection{
	Prefix: "custom",
	Commands: []string{
		"!payload",
	},
	Sounds: []*Sound{
		createSound("payload", 1, 250),
	},
}

var REDDWARF *SoundCollection = &SoundCollection{
	Prefix: "reddwarf",
	Commands: []string{
		"!reddwarf",
	},
	Sounds: []*Sound{
		createSound("deaddave", 1, 250),
	},
}

var CHOPPAH *SoundCollection = &SoundCollection{
	Prefix: "custom",
	Commands: []string{
		"!choppah",
	},
	Sounds: []*Sound{
		createSound("choppah", 1, 250),
	},
}

var SPACEMARINES *SoundCollection = &SoundCollection{
	Prefix: "custom",
	Commands: []string{
		"!spacemarines",
	},
	Sounds: []*Sound{
		createSound("spacemarines", 1, 250),
	},
}

var DUKE *SoundCollection = &SoundCollection{
	Prefix: "duke",
	Commands: []string{
		"!duke",
	},
	Sounds: []*Sound{
		createSound("ballsballsballs", 1, 250),
		createSound("ballsofsteel", 1, 250),
		createSound("blowitout", 1, 250),
		createSound("eatshit", 1, 250),
	},
}
var WICKEDSICK *SoundCollection = &SoundCollection{
	Prefix: "custom",
	Commands: []string{
		"!wickedsick",
	},
	Sounds: []*Sound{
		createSound("wickedsick", 1, 250),
	},
}
var WINNING *SoundCollection = &SoundCollection{
	Prefix: "custom",
	Commands: []string{
		"!winning",
	},
	Sounds: []*Sound{
		createSound("winning", 1, 250),
	},
}
var FUCKED *SoundCollection = &SoundCollection{
	Prefix: "custom",
	Commands: []string{
		"!yourefucked",
	},
	Sounds: []*Sound{
		createSound("yourefucked", 1, 250),
	},
}
var ADULT *SoundCollection = &SoundCollection{
	Prefix: "custom",
	Commands: []string{
		"!adult",
	},
	Sounds: []*Sound{
		createSound("adult", 1, 250),
	},
}
var AIUR *SoundCollection = &SoundCollection{
	Prefix: "custom",
	Commands: []string{
		"!aiur",
	},
	Sounds: []*Sound{
		createSound("aiur", 1, 250),
	},
}
var BROKETHERULES *SoundCollection = &SoundCollection{
	Prefix: "custom",
	Commands: []string{
		"!broketherules",
	},
	Sounds: []*Sound{
		createSound("broketherules", 1, 250),
	},
}
var BUTTER *SoundCollection = &SoundCollection{
	Prefix: "custom",
	Commands: []string{
		"!butter",
	},
	Sounds: []*Sound{
		createSound("butter", 1, 250),
	},
}
var CHANGE *SoundCollection = &SoundCollection{
	Prefix: "custom",
	Commands: []string{
		"!change",
	},
	Sounds: []*Sound{
		createSound("change", 1, 250),
	},
}
var CHINESEFOOD *SoundCollection = &SoundCollection{
	Prefix: "custom",
	Commands: []string{
		"!chinesefood",
	},
	Sounds: []*Sound{
		createSound("chinesefood", 1, 250),
	},
}
var DANGERZONE *SoundCollection = &SoundCollection{
	Prefix: "custom",
	Commands: []string{
		"!dangerzone",
	},
	Sounds: []*Sound{
		createSound("dangerzone", 1, 250),
	},
}
var DINOSAUR *SoundCollection = &SoundCollection{
	Prefix: "custom",
	Commands: []string{
		"!dinosaur",
	},
	Sounds: []*Sound{
		createSound("dinosaur", 1, 250),
	},
}
var FLUFFY *SoundCollection = &SoundCollection{
	Prefix: "custom",
	Commands: []string{
		"!fluffy",
	},
	Sounds: []*Sound{
		createSound("fluffy", 1, 250),
	},
}
var GROUND *SoundCollection = &SoundCollection{
	Prefix: "custom",
	Commands: []string{
		"!ground",
	},
	Sounds: []*Sound{
		createSound("ground", 1, 250),
	},
}
var HEDGEHOG *SoundCollection = &SoundCollection{
	Prefix: "custom",
	Commands: []string{
		"!hedgehog",
	},
	Sounds: []*Sound{
		createSound("hedgehog", 1, 250),
	},
}
var HESAYSNO *SoundCollection = &SoundCollection{
	Prefix: "custom",
	Commands: []string{
		"!hesaysno",
	},
	Sounds: []*Sound{
		createSound("hesaysno", 1, 250),
	},
}
var ITSMINE *SoundCollection = &SoundCollection{
	Prefix: "custom",
	Commands: []string{
		"!itsmine",
	},
	Sounds: []*Sound{
		createSound("itsmine", 1, 250),
		createSound("thisismine", 1, 250),
	},
}
var JUSTHADSEX *SoundCollection = &SoundCollection{
	Prefix: "custom",
	Commands: []string{
		"!justhadsex",
	},
	Sounds: []*Sound{
		createSound("justhadsex", 1, 250),
	},
}
var LIKDIS *SoundCollection = &SoundCollection{
	Prefix: "custom",
	Commands: []string{
		"!likdis",
	},
	Sounds: []*Sound{
		createSound("likdis", 1, 250),
	},
}
var PANCAKE *SoundCollection = &SoundCollection{
	Prefix: "custom",
	Commands: []string{
		"!pancake",
	},
	Sounds: []*Sound{
		createSound("pancake", 1, 250),
	},
}
var PUDDI *SoundCollection = &SoundCollection{
	Prefix: "custom",
	Commands: []string{
		"!puddi",
	},
	Sounds: []*Sound{
		createSound("puddi", 1, 250),
	},
}
var CARL *SoundCollection = &SoundCollection{
	Prefix: "carl",
	Commands: []string{
		"!carl",
	},
	Sounds: []*Sound{
		createSound("1", 1, 250),
		createSound("2", 1, 250),
		createSound("3", 1, 250),
	},
}
var PYLONS *SoundCollection = &SoundCollection{
	Prefix: "custom",
	Commands: []string{
		"!additionalpylons",
	},
	Sounds: []*Sound{
		createSound("additionalpylons", 1, 250),
	},
}
var ALWAYS *SoundCollection = &SoundCollection{
	Prefix: "custom",
	Commands: []string{
		"!always",
	},
	Sounds: []*Sound{
		createSound("always", 1, 250),
	},
}
var ANDMYAXE *SoundCollection = &SoundCollection{
	Prefix: "custom",
	Commands: []string{
		"!andmyaxe",
	},
	Sounds: []*Sound{
		createSound("andmyaxe", 1, 250),
	},
}
var CILLITBANG *SoundCollection = &SoundCollection{
	Prefix: "custom",
	Commands: []string{
		"!cillitbang",
	},
	Sounds: []*Sound{
		createSound("cillitbang", 1, 250),
	},
}
var COLDISTHEVOID *SoundCollection = &SoundCollection{
	Prefix: "custom",
	Commands: []string{
		"!coldisthevoid",
	},
	Sounds: []*Sound{
		createSound("coldisthevoid", 1, 250),
	},
}
var DISPENSER *SoundCollection = &SoundCollection{
	Prefix: "custom",
	Commands: []string{
		"!dispenserhere",
	},
	Sounds: []*Sound{
		createSound("dispenserhere", 1, 250),
	},
}
var FUCKTHEKING *SoundCollection = &SoundCollection{
	Prefix: "custom",
	Commands: []string{
		"!fucktheking",
	},
	Sounds: []*Sound{
		createSound("fucktheking", 1, 250),
	},
}
var HOSPITAL *SoundCollection = &SoundCollection{
	Prefix: "custom",
	Commands: []string{
		"!hospital",
	},
	Sounds: []*Sound{
		createSound("hospital", 1, 250),
	},
}
var KABOOM *SoundCollection = &SoundCollection{
	Prefix: "custom",
	Commands: []string{
		"!kaboom",
	},
	Sounds: []*Sound{
		createSound("kaboom", 1, 250),
	},
}
var LEATHERBELT *SoundCollection = &SoundCollection{
	Prefix: "custom",
	Commands: []string{
		"!leatherbelt",
	},
	Sounds: []*Sound{
		createSound("leatherbelt", 1, 250),
	},
}
var MSIXTEEN *SoundCollection = &SoundCollection{
	Prefix: "custom",
	Commands: []string{
		"!m16",
	},
	Sounds: []*Sound{
		createSound("m16", 1, 250),
	},
}
var OSTRICHES *SoundCollection = &SoundCollection{
	Prefix: "custom",
	Commands: []string{
		"!ostriches",
	},
	Sounds: []*Sound{
		createSound("ostriches", 1, 250),
	},
}
var PYTHON *SoundCollection = &SoundCollection{
	Prefix: "python",
	Commands: []string{
		"!python",
	},
	Sounds: []*Sound{
		createSound("spanishinquisition", 1, 250),
	},
}
var HPC *SoundCollection = &SoundCollection{
	Prefix: "hpc",
	Commands: []string{
		"!hpc",
	},
	Sounds: []*Sound{
		createSound("dangerous", 1, 250),
		createSound("hardballs", 1, 250),
	},
}
var FOX *SoundCollection = &SoundCollection{
	Prefix: "fox",
	Commands: []string{
		"!fox",
	},
	Sounds: []*Sound{
		createSound("ahuehahee", 1, 250),
		createSound("awoowoowoo", 1, 250),
		createSound("pititow", 1, 250),
		createSound("tatatow", 1, 250),
		createSound("yoyoyobito", 1, 250),
	},
}
var CHEERS *SoundCollection = &SoundCollection{
	Prefix: "custom",
	Commands: []string{
		"!cheers",
	},
	Sounds: []*Sound{
		createSound("cheerslove", 1, 250),
	},
}

var CHEERSEARS *SoundCollection = &SoundCollection{
	Prefix: "custom",
	Commands: []string{
		"!cheersears",
	},
	Sounds: []*Sound{
		createSound("cheers", 1, 250),
	},
}

var BULLETSTORM *SoundCollection = &SoundCollection{
	Prefix: "bulletstorm",
	Commands: []string{
		"!bulletstorm",
	},
	Sounds: []*Sound{
		createSound("heavenly", 1, 250),
		createSound("retard", 1, 250),
		createSound("sonofadick", 1, 250),
		createSound("Whatthedick2", 1, 250),
		createSound("Whatthedick", 1, 250),
	},
}

var MINE *SoundCollection = &SoundCollection{
	Prefix: "custom",
	Commands: []string{
		"!mine",
	},
	Sounds: []*Sound{
		createSound("mine", 1, 250),
	},
}

var MOVEBITCH *SoundCollection = &SoundCollection{
	Prefix: "custom",
	Commands: []string{
		"!movebitch",
	},
	Sounds: []*Sound{
		createSound("movebitch", 1, 250),
	},
}

var NYAN *SoundCollection = &SoundCollection{
	Prefix: "custom",
	Commands: []string{
		"!nyan",
	},
	Sounds: []*Sound{
		createSound("nyan", 1, 250),
	},
}

var ONABOAT *SoundCollection = &SoundCollection{
	Prefix: "onaboat",
	Commands: []string{
		"!onaboat",
	},
	Sounds: []*Sound{
		createSound("1", 1, 250),
		createSound("2", 1, 250),
		createSound("3", 1, 250),
		createSound("5", 1, 250),
	},
}

var HIBERNIAN *SoundCollection = &SoundCollection{
	Prefix: "Hibernian",
	Commands: []string{
		"!eggs",
		"!hibernianeggs",
	},
	Sounds: []*Sound{
		createSound("1", 1, 250),
		createSound("2", 1, 250),
	},
}

var ZAPP *SoundCollection = &SoundCollection{
	Prefix: "zapp",
	Commands: []string{
		"!zapp",
	},
	Sounds: []*Sound{
		createSound("champagne", 1, 250),
		createSound("checkmate", 1, 250),
		createSound("chess", 1, 250),
		createSound("exploding", 1, 250),
		createSound("fighting", 1, 250),
		createSound("flag", 1, 250),
		createSound("gravity", 1, 250),
		createSound("lower", 1, 250),
		createSound("sexfully", 1, 250),
		createSound("sexlexia", 1, 250),
		createSound("woman", 1, 250),
	},
}
var ONYXIA *SoundCollection = &SoundCollection{
	Prefix: "onyxia",
	Commands: []string{
		"!onyxia",
	},
	Sounds: []*Sound{
		createSound("aggro", 1, 250),
	},
}
var SHAME *SoundCollection = &SoundCollection{
	Prefix: "shame",
	Commands: []string{
		"!shame",
	},
	Sounds: []*Sound{
		createSound("shame", 1, 250),
	},
}
var VICTORY *SoundCollection = &SoundCollection{
	Prefix: "custom",
	Commands: []string{
		"!victory",
	},
	Sounds: []*Sound{
		createSound("victory", 1, 250),
	},
}



var COLLECTIONS []*SoundCollection = []*SoundCollection{
	AIRHORN,
	KHALED,
	CENA,
	ETHAN,
	COW,
	LOWETHAN,
	HIGHETHAN,
	MULTIETHAN,
	CENANEW,
	CAPTAIN,
	JEFF,
	ETHANCENA,
	FUSRODAH,
	BASE,
	LEEROY,
	LEEROYFULL,
	AKBAR,
	MAD,
	CODEC,
	NTHOU,
	NTHOUFULL,
	SURPRISE,
	WOMBO,
	LOSE,
	SPOOKY,
	ERRYDAY,
	EMINEM,
	DARKNESS,
	JOHNCENA,
	//WORLD,
	YOULOSE,
	ALLIGATOR,
	NOTIME,
	PICCOLO,
	SHIA,
	MARIO,
	FFFF,
	NEUTRAL,
	UMAD,
	RICK,
	FEELSBAD,
	TWISTED,
	NIC,
	ERANU,
	FMJ,
	IDEAGAY,
	HAHGAY,
	SMOKE,
	OVERWATCH,
	JABRONI,
	MUDA,
	FEVER,
	COMEWITHME,
	BRERBRER,
	COOKIE,
	PAYLOAD,
	REDDWARF,
	CHOPPAH,
	SPACEMARINES,
	DUKE,
	WICKEDSICK,
	WINNING,
	FUCKED,
	ADULT,
	AIUR,
	BROKETHERULES,
	BUTTER,
	CHANGE,
	CHINESEFOOD,
	DANGERZONE,
	DINOSAUR,
	FLUFFY,
	GROUND,
	HEDGEHOG,
	HESAYSNO,
	ITSMINE,
	JUSTHADSEX,
	LIKDIS,
	PANCAKE,
	PUDDI,
	CARL,
	PYLONS,
	ALWAYS,
	ANDMYAXE,
	CILLITBANG,
	COLDISTHEVOID,
	DISPENSER,
	FUCKTHEKING,
	HOSPITAL,
	KABOOM,
	LEATHERBELT,
	MSIXTEEN,
	OSTRICHES,
	PYTHON,
	HPC,
	FOX,
	CHEERS,
	CHEERSEARS,
	BULLETSTORM,
	MINE,
	MOVEBITCH,
	NYAN,
	ONABOAT,
	HIBERNIAN,
	ZAPP,
	ONYXIA,
	SHAME,
	VICTORY,
}
// Add Sound Above


// Create a Sound struct
func createSound(Name string, Weight int, PartDelay int) *Sound {
	return &Sound{
		Name:      Name,
		Weight:    Weight,
		PartDelay: PartDelay,
		buffer:    make([][]byte, 0),
	}
}

func (sc *SoundCollection) Load() {
	for _, sound := range sc.Sounds {
		sc.soundRange += sound.Weight
		sound.Load(sc)
	}
}

func (s *SoundCollection) Random() *Sound {
	var (
		i      int
		number int = randomRange(0, s.soundRange)
	)

	for _, sound := range s.Sounds {
		i += sound.Weight

		if number < i {
			return sound
		}
	}
	return nil
}

// Load attempts to load an encoded sound file from disk
// DCA files are pre-computed sound files that are easy to send to Discord.
// If you would like to create your own DCA files, please use:
// https://github.com/nstafie/dca-rs
// eg: dca-rs --raw -i <input wav file> > <output file>
func (s *Sound) Load(c *SoundCollection) error {
	path := fmt.Sprintf("audio/%v_%v.dca", c.Prefix, s.Name)

	file, err := os.Open(path)

	if err != nil {
		fmt.Println("error opening dca file :", err)
		return err
	}

	var opuslen int16

	for {
		// read opus frame length from dca file
		err = binary.Read(file, binary.LittleEndian, &opuslen)

		// If this is the end of the file, just return
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			return nil
		}

		if err != nil {
			fmt.Println("error reading from dca file :", err)
			return err
		}

		// read encoded pcm from dca file
		InBuf := make([]byte, opuslen)
		err = binary.Read(file, binary.LittleEndian, &InBuf)

		// Should not be any end of file errors
		if err != nil {
			fmt.Println("error reading from dca file :", err)
			return err
		}

		// append encoded pcm data to the buffer
		s.buffer = append(s.buffer, InBuf)
	}
}

// Plays this sound over the specified VoiceConnection
func (s *Sound) Play(vc *discordgo.VoiceConnection) {
	vc.Speaking(true)
	defer vc.Speaking(false)

	for _, buff := range s.buffer {
		vc.OpusSend <- buff
	}
}

// Attempts to find the current users voice channel inside a given guild
func getCurrentVoiceChannel(user *discordgo.User, guild *discordgo.Guild) *discordgo.Channel {
	for _, vs := range guild.VoiceStates {
		if vs.UserID == user.ID {
			channel, _ := discord.State.Channel(vs.ChannelID)
			return channel
		}
	}
	return nil
}

// Whether a guild id is in this shard
func shardContains(guildid string) bool {
	if len(SHARDS) != 0 {
		ok := false
		for _, shard := range SHARDS {
			if len(guildid) >= 5 && string(guildid[len(guildid)-5]) == shard {
				ok = true
				break
			}
		}
		return ok
	}
	return true
}

// Returns a random integer between min and max
func randomRange(min, max int) int {
	rand.Seed(time.Now().UTC().UnixNano())
	return rand.Intn(max-min) + min
}

// Prepares a play
func createPlay(user *discordgo.User, guild *discordgo.Guild, coll *SoundCollection, sound *Sound) *Play {
	// Grab the users voice channel
	channel := getCurrentVoiceChannel(user, guild)
	if channel == nil {
		log.WithFields(log.Fields{
			"user":  user.ID,
			"guild": guild.ID,
		}).Warning("Failed to find channel to play sound in")
		return nil
	}

	// Create the play
	play := &Play{
		GuildID:   guild.ID,
		ChannelID: channel.ID,
		UserID:    user.ID,
		Sound:     sound,
		Forced:    true,
	}

	// If we didn't get passed a manual sound, generate a random one
	if play.Sound == nil {
		play.Sound = coll.Random()
		play.Forced = false
	}

	// If the collection is a chained one, set the next sound
	if coll.ChainWith != nil {
		play.Next = &Play{
			GuildID:   play.GuildID,
			ChannelID: play.ChannelID,
			UserID:    play.UserID,
			Sound:     coll.ChainWith.Random(),
			Forced:    play.Forced,
		}
	}

	return play
}

// Prepares and enqueues a play into the ratelimit/buffer guild queue
func enqueuePlay(user *discordgo.User, guild *discordgo.Guild, coll *SoundCollection, sound *Sound) {
	play := createPlay(user, guild, coll, sound)
	if play == nil {
		return
	}

	// Check if we already have a connection to this guild
	//   yes, this isn't threadsafe, but its "OK" 99% of the time
	_, exists := queues[guild.ID]

	if exists {
		if len(queues[guild.ID]) < MAX_QUEUE_SIZE {
			queues[guild.ID] <- play
		}
	} else {
		queues[guild.ID] = make(chan *Play, MAX_QUEUE_SIZE)
		playSound(play, nil)
	}
}

func trackSoundStats(play *Play) {
	if rcli == nil {
		return
	}

	_, err := rcli.Pipelined(func(pipe *redis.Pipeline) error {
		var baseChar string

		if play.Forced {
			baseChar = "f"
		} else {
			baseChar = "a"
		}

		base := fmt.Sprintf("airhorn:%s", baseChar)
		pipe.Incr("airhorn:total")
		pipe.Incr(fmt.Sprintf("%s:total", base))
		pipe.Incr(fmt.Sprintf("%s:sound:%s", base, play.Sound.Name))
		pipe.Incr(fmt.Sprintf("%s:user:%s:sound:%s", base, play.UserID, play.Sound.Name))
		pipe.Incr(fmt.Sprintf("%s:guild:%s:sound:%s", base, play.GuildID, play.Sound.Name))
		pipe.Incr(fmt.Sprintf("%s:guild:%s:chan:%s:sound:%s", base, play.GuildID, play.ChannelID, play.Sound.Name))
		pipe.SAdd(fmt.Sprintf("%s:users", base), play.UserID)
		pipe.SAdd(fmt.Sprintf("%s:guilds", base), play.GuildID)
		pipe.SAdd(fmt.Sprintf("%s:channels", base), play.ChannelID)
		return nil
	})

	if err != nil {
		log.WithFields(log.Fields{
			"error": err,
		}).Warning("Failed to track stats in redis")
	}
}

// Play a sound
func playSound(play *Play, vc *discordgo.VoiceConnection) (err error) {
	log.WithFields(log.Fields{
		"play": play,
	}).Info("Playing sound")

	if vc == nil {
		vc, err = discord.ChannelVoiceJoin(play.GuildID, play.ChannelID, false, false)
		// vc.Receive = false
		if err != nil {
			log.WithFields(log.Fields{
				"error": err,
			}).Error("Failed to play sound")
			delete(queues, play.GuildID)
			return err
		}
	}

	// If we need to change channels, do that now
	if vc.ChannelID != play.ChannelID {
		vc.ChangeChannel(play.ChannelID, false, false)
		time.Sleep(time.Millisecond * 125)
	}

	// Track stats for this play in redis
	go trackSoundStats(play)

	// Sleep for a specified amount of time before playing the sound
	time.Sleep(time.Millisecond * 32)

	// Play the sound
	play.Sound.Play(vc)

	// If this is chained, play the chained sound
	if play.Next != nil {
		playSound(play.Next, vc)
	}

	// If there is another song in the queue, recurse and play that
	if len(queues[play.GuildID]) > 0 {
		play := <-queues[play.GuildID]
		playSound(play, vc)
		return nil
	}

	// If the queue is empty, delete it
	time.Sleep(time.Millisecond * time.Duration(play.Sound.PartDelay))
	delete(queues, play.GuildID)
	vc.Disconnect()
	return nil
}

func onReady(s *discordgo.Session, event *discordgo.Ready) {
	log.Info("Recieved READY payload")
}

func onGuildCreate(s *discordgo.Session, event *discordgo.GuildCreate) {
	if !shardContains(event.Guild.ID) {
		return
	}

	if event.Guild.Unavailable != nil {
		return
	}

	for _, channel := range event.Guild.Channels {
		if channel.ID == event.Guild.ID {
			s.ChannelMessageSend(channel.ID, "**AIRHORN BOT READY FOR HORNING. TYPE `!AIRHORN` WHILE IN A VOICE CHANNEL TO ACTIVATE**")
			return
		}
	}
}

func scontains(key string, options ...string) bool {
	for _, item := range options {
		if item == key {
			return true
		}
	}
	return false
}

func calculateAirhornsPerSecond(cid string) {
	current, _ := strconv.Atoi(rcli.Get("airhorn:a:total").Val())
	time.Sleep(time.Second * 10)
	latest, _ := strconv.Atoi(rcli.Get("airhorn:a:total").Val())

	discord.ChannelMessageSend(cid, fmt.Sprintf("Current APS: %v", (float64(latest-current))/10.0))
}

func displayBotStats(cid string) {
	stats := runtime.MemStats{}
	runtime.ReadMemStats(&stats)

	users := 0
	for _, guild := range discord.State.Ready.Guilds {
		users += len(guild.Members)
	}

	w := &tabwriter.Writer{}
	buf := &bytes.Buffer{}

	w.Init(buf, 0, 4, 0, ' ', 0)
	fmt.Fprintf(w, "```\n")
	fmt.Fprintf(w, "Discordgo: \t%s\n", discordgo.VERSION)
	fmt.Fprintf(w, "Go: \t%s\n", runtime.Version())
	fmt.Fprintf(w, "Memory: \t%s / %s (%s total allocated)\n", humanize.Bytes(stats.Alloc), humanize.Bytes(stats.Sys), humanize.Bytes(stats.TotalAlloc))
	fmt.Fprintf(w, "Tasks: \t%d\n", runtime.NumGoroutine())
	fmt.Fprintf(w, "Servers: \t%d\n", len(discord.State.Ready.Guilds))
	fmt.Fprintf(w, "Users: \t%d\n", users)
	fmt.Fprintf(w, "Shards: \t%s\n", strings.Join(SHARDS, ", "))
	fmt.Fprintf(w, "```\n")
	w.Flush()
	discord.ChannelMessageSend(cid, buf.String())
}

func utilSumRedisKeys(keys []string) int {
	results := make([]*redis.StringCmd, 0)

	rcli.Pipelined(func(pipe *redis.Pipeline) error {
		for _, key := range keys {
			results = append(results, pipe.Get(key))
		}
		return nil
	})

	var total int
	for _, i := range results {
		t, _ := strconv.Atoi(i.Val())
		total += t
	}

	return total
}

func displayUserStats(cid, uid string) {
	keys, err := rcli.Keys(fmt.Sprintf("airhorn:*:user:%s:sound:*", uid)).Result()
	if err != nil {
		return
	}

	totalAirhorns := utilSumRedisKeys(keys)
	discord.ChannelMessageSend(cid, fmt.Sprintf("Total Airhorns: %v", totalAirhorns))
}

func displayServerStats(cid, sid string) {
	keys, err := rcli.Keys(fmt.Sprintf("airhorn:*:guild:%s:sound:*", sid)).Result()
	if err != nil {
		return
	}

	totalAirhorns := utilSumRedisKeys(keys)
	discord.ChannelMessageSend(cid, fmt.Sprintf("Total Airhorns: %v", totalAirhorns))
}

func utilGetMentioned(s *discordgo.Session, m *discordgo.MessageCreate) *discordgo.User {
	for _, mention := range m.Mentions {
		if mention.ID != s.State.Ready.User.ID {
			return mention
		}
	}
	return nil
}

func airhornBomb(cid string, guild *discordgo.Guild, user *discordgo.User, cs string, horn string) {
	count, _ := strconv.Atoi(cs)
	discord.ChannelMessageSend(cid, ":ok_hand:"+strings.Repeat(":trumpet:", count))

	// Cap it at something
	if count > 100 {
		return
	}
	var toPlay *SoundCollection
	for _, coll := range COLLECTIONS {
		if scontains(horn, coll.Commands...) {
			toPlay = coll
			break;
		}
	}
	if (toPlay == nil) {
		return
	}

	play := createPlay(user, guild, toPlay, nil)
	vc, err := discord.ChannelVoiceJoin(play.GuildID, play.ChannelID, true, true)
	if err != nil {
		return
	}

	for i := 0; i < count; i++ {
		toPlay.Random().Play(vc)
		if (horn == "!shame") {
			AIRHORN.Random().Play(vc)
		}
	}

	vc.Disconnect()
}

// Handles bot operator messages, should be refactored (lmao)
func handleBotControlMessages(s *discordgo.Session, m *discordgo.MessageCreate, parts []string, g *discordgo.Guild) {
	ourShard := shardContains(g.ID)

	if scontains(parts[1], "status") && ourShard {
		displayBotStats(m.ChannelID)
	} else if scontains(parts[1], "stats") && ourShard {
		if len(m.Mentions) >= 2 {
			displayUserStats(m.ChannelID, utilGetMentioned(s, m).ID)
		} else if len(parts) >= 3 {
			displayUserStats(m.ChannelID, parts[2])
		} else {
			displayServerStats(m.ChannelID, g.ID)
		}
	} else if scontains(parts[1], "bomb") && len(parts) >= 4 && ourShard {
		var horn = "!horn"
		if len(parts) >= 5 {
			horn = parts[4]
		}
		airhornBomb(m.ChannelID, g, utilGetMentioned(s, m), parts[3], horn)
	} else if scontains(parts[1], "shards") {
		guilds := 0
		for _, guild := range s.State.Ready.Guilds {
			if shardContains(guild.ID) {
				guilds += 1
			}
		}
		s.ChannelMessageSend(m.ChannelID, fmt.Sprintf(
			"Shard %v contains %v servers",
			strings.Join(SHARDS, ","),
			guilds))
	} else if scontains(parts[1], "aps") && ourShard {
		s.ChannelMessageSend(m.ChannelID, ":ok_hand: give me a sec m8")
		go calculateAirhornsPerSecond(m.ChannelID)
	} else if scontains(parts[len(parts)-1], "where") && ourShard {
		s.ChannelMessageSend(m.ChannelID,
			fmt.Sprintf("its a me, shard %v", string(g.ID[len(g.ID)-5])))
	} else if scontains(parts[1], "changestatus") && ourShard {
		statusArr := strings.Split(m.Content, " ")
		status := strings.Join(statusArr[2:len(statusArr)], " ")
		s.UpdateStatus(0, status)
	} else if scontains(parts[1], "echo") && ourShard {
		statusArr := strings.Split(m.Content, " ")
		status := strings.Join(statusArr[2:len(statusArr)], " ")
		s.ChannelMessageSend(m.ChannelID, status)
	}
	return
}

func onMessageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	if len(m.Content) <= 0 || (m.Content[0] != '!' && len(m.Mentions) < 1) {
		return
	}

	msg := strings.Replace(m.ContentWithMentionsReplaced(), s.State.Ready.User.Username, "username", 1)
	parts := strings.Split(strings.ToLower(msg), " ")

	channel, _ := discord.State.Channel(m.ChannelID)
	if channel == nil {
		log.WithFields(log.Fields{
			"channel": m.ChannelID,
			"message": m.ID,
		}).Warning("Failed to grab channel")
		return
	}

	guild, _ := discord.State.Guild(channel.GuildID)
	if guild == nil {
		log.WithFields(log.Fields{
			"guild":   channel.GuildID,
			"channel": channel,
			"message": m.ID,
		}).Warning("Failed to grab guild")
		return
	}

	// If this is a mention, it should come from the owner (otherwise we don't care)
	if len(m.Mentions) > 0 && m.Author.ID == OWNER && len(parts) > 0 {
		mentioned := false
		for _, mention := range m.Mentions {
			mentioned = (mention.ID == s.State.Ready.User.ID)
			if mentioned {
				break
			}
		}

		if mentioned {
			handleBotControlMessages(s, m, parts, guild)
		}
		return
	}

	// If it's not relevant to our shard, just exit
	if !shardContains(guild.ID) {
		return
	}

	if (parts[0] == "!listcommands") {
		var allCommands []string
		for _, coll := range COLLECTIONS {
			allCommands = append(allCommands, coll.Commands...);
		}
		var outputString string
		outputString = "Command List: " + strings.Join(allCommands, " ")
		s.ChannelMessageSend(m.ChannelID, outputString)
	}

	// Find the collection for the command we got
	for _, coll := range COLLECTIONS {
		if scontains(parts[0], coll.Commands...) {

			// If they passed a specific sound effect, find and select that (otherwise play nothing)
			var sound *Sound
			if len(parts) > 1 {
				for _, s := range coll.Sounds {
					if parts[1] == s.Name {
						sound = s
					}
				}

				if sound == nil {
					return
				}
			}

			go enqueuePlay(m.Author, guild, coll, sound)
			s.ChannelMessageDelete(m.ChannelID, m.ID)
			return
		}
	}
}

func main() {
	var (
		Token = flag.String("t", "", "Discord Authentication Token")
		Redis = flag.String("r", "", "Redis Connection String")
		Shard = flag.String("s", "", "Integers to shard by")
		Owner = flag.String("o", "", "Owner ID")
		err   error
	)
	flag.Parse()

	if *Owner != "" {
		OWNER = *Owner
	}

	// Make sure shard is either empty, or an integer
	if *Shard != "" {
		SHARDS = strings.Split(*Shard, ",")

		for _, shard := range SHARDS {
			if _, err := strconv.Atoi(shard); err != nil {
				log.WithFields(log.Fields{
					"shard": shard,
					"error": err,
				}).Fatal("Invalid Shard")
				return
			}
		}
	}

	// Preload all the sounds
	log.Info("Preloading sounds...")
	for _, coll := range COLLECTIONS {
		coll.Load()
	}

	// If we got passed a redis server, try to connect
	if *Redis != "" {
		log.Info("Connecting to redis...")
		rcli = redis.NewClient(&redis.Options{Addr: *Redis, DB: 0})
		_, err = rcli.Ping().Result()

		if err != nil {
			log.WithFields(log.Fields{
				"error": err,
			}).Fatal("Failed to connect to redis")
			return
		}
	}

	// Create a discord session
	log.Info("Starting discord session...")
	discord, err = discordgo.New(*Token)
	if err != nil {
		log.WithFields(log.Fields{
			"error": err,
		}).Fatal("Failed to create discord session")
		return
	}

	discord.AddHandler(onReady)
	discord.AddHandler(onGuildCreate)
	discord.AddHandler(onMessageCreate)

	err = discord.Open()
	if err != nil {
		log.WithFields(log.Fields{
			"error": err,
		}).Fatal("Failed to create discord websocket connection")
		return
	}

	// We're running!
	log.Info("AIRHORNBOT is ready to horn it up.")

	// Wait for a signal to quit
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, os.Kill)
	<-c
}
