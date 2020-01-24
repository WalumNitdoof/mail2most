package mail2most

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"time"
)

// Run starts mail2most
func (m Mail2Most) Run() error {
	alreadySend := make([][]uint32, len(m.Config.Profiles))
	alreadySendFile := make([][]uint32, len(m.Config.Profiles))
	if _, err := os.Stat(m.Config.General.File); err == nil {
		jsonFile, err := os.Open(m.Config.General.File)
		if err != nil {
			return err
		}

		bv, err := ioutil.ReadAll(jsonFile)
		if err != nil {
			return err
		}

		err = json.Unmarshal(bv, &alreadySendFile)
		if err != nil {
			return err
		}
	}

	// write cache to memory cache
	// this is nessasary if new profiles are added
	// and the caching file does not contain any caching
	// for this profile
	l := len(alreadySend)
	for k, v := range alreadySendFile {
		if k < l {
			alreadySend[k] = v
		}
		if k >= l {
			m.Error("data.json error", map[string]interface{}{
				"error":    "data.json contains more profile information than defined in the config",
				"cause":    "this happens if profiles are deleted from the config file and can create inconsistencies",
				"solution": "delete the data.json file",
				"note":     "by deleting the data.json file all mails are parsed and send again",
			})
		}
	}

	// set a 10 seconds sleep default if no TimeInterval is defined
	if m.Config.General.TimeInterval == 0 {
		m.Info("no check time interval set", map[string]interface{}{
			"fallback":     10,
			"unit-of-time": "second",
		})
		m.Config.General.TimeInterval = 10
	}

	for {
		for p := range m.Config.Profiles {
			c, err := m.connect(p)
			if err != nil {
				m.Error("Error connecting mailserver", map[string]interface{}{
					"Error":  err,
					"Server": m.Config.Profiles[p].Mail.ImapServer,
				})
			}
			defer c.Logout()

			mails, err := m.GetMail(p, c)
			if err != nil {
				m.Error("Error reaching mailserver", map[string]interface{}{
					"Error":  err,
					"Server": m.Config.Profiles[p].Mail.ImapServer,
				})
				break
			}

			for _, mail := range mails {
				send := true
				for _, id := range alreadySend[p] {
					if mail.ID == id {
						m.Debug("mail", map[string]interface{}{
							"subject":    mail.Subject,
							"status":     "already send",
							"message-id": mail.ID,
						})
						send = false
					}
				}
				if send {
					err := m.PostMattermost(p, mail)
					if err != nil {
						m.Error("Mattermost Error", map[string]interface{}{
							"Error": err,
						})
					} else {
						alreadySend[p] = append(alreadySend[p], mail.ID)
					}
					err = writeToFile(alreadySend, m.Config.General.File)
					if err != nil {
						return err
					}

				}
			}
		}
		//time.Sleep(time.Duration(m.Config.General.TimeInterval) * 10 * time.Second)
		m.Debug("sleeping", map[string]interface{}{
			"intervaltime": m.Config.General.TimeInterval,
			"unit-of-time": "second",
		})
		time.Sleep(time.Duration(m.Config.General.TimeInterval) * time.Second)
	}
}
