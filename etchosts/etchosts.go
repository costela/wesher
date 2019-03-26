package etchosts

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"strings"

	log "github.com/sirupsen/logrus"
)

// DefaultBanner is the default magic comment used to identify entries managed by etchosts
const DefaultBanner = "# ! MANAGED AUTOMATICALLY !"

// DefaultPath is the default path used to write hosts entries
const DefaultPath = "/etc/hosts"

// EtcHosts contains the options used to write hosts entries.
// The zero value can be used to write to DefaultPath using DefaultBanner as a marker.
type EtcHosts struct {
	// Banner is the magic comment used to identify entries managed by etchosts; if not set, will use DefaultBanner.
	// It must start with "#" to mark it as a comment.
	Banner string
	// Path is the path to the /etc/hosts file; if not set, will use DefaultPath.
	Path string
	// Logger is an optional logrus.StdLogger interface, used for debugging.
	Logger log.StdLogger
}

// WriteEntries is used to write the hosts entries to EtcHosts.Path
// Each IP address with their (potentially multiple) hostnames are written to a line marked with EtcHosts.Banner, to
// avoid overwriting preexisting entries.
func (eh *EtcHosts) WriteEntries(ipsToNames map[string][]string) error {
	hostsPath := eh.Path
	if hostsPath == "" {
		hostsPath = DefaultPath
	}

	// We do not want to create the hosts file; if it's not there, we probably have the wrong path.
	etcHosts, err := os.OpenFile(hostsPath, os.O_RDWR, 0644)
	if err != nil {
		return fmt.Errorf("could not open %s for reading: %s", hostsPath, err)
	}
	defer etcHosts.Close()

	// create tmpfile in same folder as
	tmp, err := ioutil.TempFile(path.Dir(hostsPath), "etchosts")
	if err != nil {
		return fmt.Errorf("could not create tempfile")
	}

	// remove tempfile; this might fail if we managed to move it, which is ok
	defer func(file *os.File) {
		file.Close()
		if err := os.Remove(file.Name()); err != nil && !os.IsNotExist(err) {
			if eh.Logger != nil {
				eh.Logger.Printf("unexpected error trying to remove temp file %s: %s", file.Name(), err)
			}
		}
	}(tmp)

	if err := eh.writeEntries(etcHosts, tmp, ipsToNames); err != nil {
		return err
	}

	return eh.movePreservePerms(tmp, etcHosts)
}

func (eh *EtcHosts) writeEntries(orig io.Reader, dest io.Writer, ipsToNames map[string][]string) error {
	banner := eh.Banner
	if banner == "" {
		banner = DefaultBanner
	}

	// go through file and update existing entries/prune nonexistent entries
	scanner := bufio.NewScanner(orig)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasSuffix(strings.TrimSpace(line), strings.TrimSpace(banner)) {
			tokens := strings.Fields(line)
			if len(tokens) < 1 {
				continue // remove empty managed line
			}
			ip := tokens[0]
			if names, ok := ipsToNames[ip]; ok {
				err := eh.writeEntryWithBanner(dest, banner, ip, names)
				if err != nil {
					return err
				}
				delete(ipsToNames, ip) // otherwise we'll append it again below
			}
		} else {
			// keep original unmanaged line
			fmt.Fprintf(dest, "%s\n", line)
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading hosts file: %s", err)
	}

	// append remaining entries to file
	for ip, names := range ipsToNames {
		if err := eh.writeEntryWithBanner(dest, banner, ip, names); err != nil {
			return err
		}
	}

	return nil
}

func (eh *EtcHosts) writeEntryWithBanner(tmp io.Writer, banner, ip string, names []string) error {
	if ip != "" && len(names) > 0 {
		if eh.Logger != nil {
			eh.Logger.Printf("writing entry for %s (%s)", ip, names)
		}
		if _, err := fmt.Fprintf(tmp, "%s\t%s\t%s\n", ip, strings.Join(names, " "), banner); err != nil {
			return fmt.Errorf("error writing entry for %s: %s", ip, err)
		}
	}
	return nil
}

func (eh *EtcHosts) movePreservePerms(src, dst *os.File) error {
	if err := src.Sync(); err != nil {
		return fmt.Errorf("could not sync changes to %s: %s", src.Name(), err)
	}

	etcHostsInfo, err := dst.Stat()
	if err != nil {
		return fmt.Errorf("could not stat %s: %s", dst.Name(), err)
	}

	if err = os.Rename(src.Name(), dst.Name()); err != nil {
		log.Infof("could not rename to %s; falling back to copy (%s)", dst.Name(), err)

		if _, err := src.Seek(0, io.SeekStart); err != nil {
			return err
		}
		if _, err := dst.Seek(0, io.SeekStart); err != nil {
			return err
		}
		if err := dst.Truncate(0); err != nil {
			return err
		}
		_, err = io.Copy(dst, src)
		return err
	}

	// ensure we're not running with some umask that might break things

	if err := src.Chmod(etcHostsInfo.Mode()); err != nil {
		return fmt.Errorf("could not chmod %s: %s", src.Name(), err)
	}
	// TODO: also keep user?

	return nil
}
