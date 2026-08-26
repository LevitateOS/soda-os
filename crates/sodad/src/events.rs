use std::{
    collections::HashSet,
    sync::{
        Arc, Mutex,
        atomic::{AtomicU64, Ordering},
    },
    time::Duration,
};

use soda_core::{EventKind, SodaEvent};
use tokio::sync::broadcast;
use uuid::Uuid;

#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash)]
struct EventKey {
    kind: EventKind,
    project_id: Option<Uuid>,
}

#[derive(Clone)]
pub struct EventBroker {
    inner: Arc<Inner>,
}

struct Inner {
    sender: broadcast::Sender<SodaEvent>,
    sequence: AtomicU64,
    pending: Mutex<HashSet<EventKey>>,
}

impl EventBroker {
    pub fn new() -> Self {
        let (sender, _) = broadcast::channel(256);
        Self {
            inner: Arc::new(Inner {
                sender,
                sequence: AtomicU64::new(0),
                pending: Mutex::new(HashSet::new()),
            }),
        }
    }

    pub fn subscribe(&self) -> broadcast::Receiver<SodaEvent> {
        self.inner.sender.subscribe()
    }

    pub fn publish(&self, kind: EventKind, project_id: Option<Uuid>) {
        let key = EventKey { kind, project_id };
        if tokio::runtime::Handle::try_current().is_err() {
            self.send(key);
            return;
        }
        {
            let mut pending = self.inner.pending.lock().expect("event mutex poisoned");
            if !pending.insert(key) {
                return;
            }
        }
        let broker = self.clone();
        tokio::spawn(async move {
            tokio::time::sleep(Duration::from_millis(250)).await;
            broker
                .inner
                .pending
                .lock()
                .expect("event mutex poisoned")
                .remove(&key);
            broker.send(key);
        });
    }

    fn send(&self, key: EventKey) {
        let sequence = self.inner.sequence.fetch_add(1, Ordering::Relaxed) + 1;
        let _ = self.inner.sender.send(SodaEvent {
            kind: key.kind,
            project_id: key.project_id,
            sequence,
        });
    }
}

impl Default for EventBroker {
    fn default() -> Self {
        Self::new()
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[tokio::test]
    async fn coalesces_duplicate_events() {
        let broker = EventBroker::new();
        let mut receiver = broker.subscribe();
        broker.publish(EventKind::PeopleChanged, None);
        broker.publish(EventKind::PeopleChanged, None);
        let event = receiver.recv().await.expect("event");
        assert_eq!(event.kind, EventKind::PeopleChanged);
        assert!(
            tokio::time::timeout(Duration::from_millis(300), receiver.recv())
                .await
                .is_err()
        );
    }
}
