package kubernetes

import (
	"context"
	"time"

	"github.com/rs/zerolog/log"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
)

func (c *Client) StartRepositoryNotifier(options *WatchCallbacks) error {
	ctx := context.Background()

	go func() {
		for {
			watcher, err := c.ClientSet.CoreV1().Secrets(c.Options.Namespace).Watch(ctx, metav1.ListOptions{
				LabelSelector: "go-cryo.github.com/repository=true",
			})
			if err != nil {
				log.Error().Err(err).Msg("failed to start repository watcher")
				time.Sleep(5 * time.Second)
				continue
			}
			for event := range watcher.ResultChan() {
				secret, ok := event.Object.(*corev1.Secret)
				if !ok {
					continue
				}

				name := secret.ObjectMeta.Name
				namespace := secret.ObjectMeta.Namespace

				log.Info().Msgf("repository secret %s/%s event: %s", namespace, name, event.Type)

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
			log.Warn().Msg("repository watcher channel closed, reconnecting...")
			time.Sleep(5 * time.Second)
		}
	}()

	return nil
}
