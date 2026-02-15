package kubernetes

import (
	"context"
	"time"

	"github.com/rs/zerolog/log"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
)

func (c *Client) StartBackupJobNotifier(options *WatchCallbacks) error {
	ctx := context.Background()

	go func() {
		for {
			watcher, err := c.ClientSet.CoreV1().ConfigMaps(c.Options.Namespace).Watch(ctx, metav1.ListOptions{
				LabelSelector: "go-cryo.github.com/config=true",
			})
			if err != nil {
				log.Error().Err(err).Msgf("failed to start backup job watcher")
				time.Sleep(5 * time.Second)
				continue
			}
			for event := range watcher.ResultChan() {
				configMap, ok := event.Object.(*corev1.ConfigMap)
				if !ok {
					continue
				}

				name := configMap.ObjectMeta.Name
				namespace := configMap.ObjectMeta.Namespace

				log.Info().Msgf("backup job config map %s/%s event: %s", namespace, name, event.Type)

				switch event.Type {
				case watch.Added:
					if options.OnAdd != nil {
						options.OnAdd(namespace, name)
					}
				case watch.Modified:
					if options.OnUpdate != nil {
						options.OnUpdate(namespace, name)
					}
				case watch.Deleted:
					if options.OnDelete != nil {
						options.OnDelete(namespace, name)
					}
				}
			}
			log.Warn().Msg("backup job watcher channel closed, reconnecting...")
			time.Sleep(5 * time.Second)
		}
	}()

	return nil
}
