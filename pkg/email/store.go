package email

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/SocialRoots/sr-email/settings"
)

func SaveRaw(raw []byte, timestamp, recipient string) {
	dir := settings.EmailStoreDir
	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Printf("[sr-email] failed to create email store dir %s: %v", dir, err)
		return
	}

	ts := timestamp
	if i, err := time.Parse(time.RFC3339, timestamp); err == nil {
		ts = i.Format("20060102T150405")
	} else {
		ts = time.Now().UTC().Format("20060102T150405")
	}

	localPart := strings.SplitN(recipient, "@", 2)[0]
	if localPart == "" {
		localPart = "unknown"
	}

	filename := fmt.Sprintf("%s_%s.eml", ts, localPart)
	path := filepath.Join(dir, filename)

	if err := os.WriteFile(path, raw, 0644); err != nil {
		log.Printf("[sr-email] failed to write email file %s: %v", path, err)
		return
	}

	log.Printf("[sr-email] saved raw email to %s (%d bytes)", path, len(raw))
}