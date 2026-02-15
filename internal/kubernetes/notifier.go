package kubernetes

type WatchCallbacks struct {
	OnAdd    func(namespace, name string)
	OnUpdate func(namespace, name string)
	OnDelete func(namespace, name string)
}

type NotifierOptions struct {
	BackupJob  *WatchCallbacks
	Repository *WatchCallbacks
}

func (c *Client) StartNotifier(options *NotifierOptions) error {
	if options.BackupJob != nil {
		if err := c.StartBackupJobNotifier(options.BackupJob); err != nil {
			return err
		}
	}
	if options.Repository != nil {
		if err := c.StartRepositoryNotifier(options.Repository); err != nil {
			return err
		}
	}
	return nil
}
