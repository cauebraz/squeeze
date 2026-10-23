# kubectl

Input: the JSON that `kubectl get <resources> -o json` prints: a `List` with the objects under `items`, or a single object when one is named.

## Agent line

For the project's `AGENTS.md` or `CLAUDE.md`:

```
- Read Kubernetes state as `kubectl get pods,deploy,jobs -o json | squeeze` and events as `kubectl get events -o json | squeeze`;
  Secret and ConfigMap values and annotations are never printed, only their keys.
```

## Example

```
kubectl get pods,deploy,jobs,svc -o json | squeeze
kubectl get all -A -o json | squeeze --fail-on fail
kubectl get events -o json | squeeze
kubectl get secret,configmap -n shop -o json | squeeze
```

`kubectl get -o json` prints every object whole: the spec, the pod template, the metadata without `managedFields` but with its annotations, and the status with every condition and timestamp.
squeeze keeps one line per object with the health that matters for its kind, unhealthy objects first, and one line per Warning event.
The fixtures were captured from a kind cluster (kubectl 1.36.1, Kubernetes 1.33.1).
The text is 1.0 to 3.0 percent of the input for the lists, 1.1 to 2.2 percent for one node or pod, and 13 percent for a 768-byte Secret.
`generic` printed 8,703 bytes of the 89,165-byte `mixed` list, against 1,184, and printed the Secret values of the `secrets` fixture.

A typed list such as `PodList` from `kubectl get --raw` is read too; its items take the list's kind.

- Detection: the input, ignoring whitespace, starts with `{"apiVersion":` and holds the keys `kind` and `metadata`. kubectl prints the keys of every object sorted.

## Output

```
$ squeeze < internal/formats/kubectl/testdata/mixed/input.json
kubectl: 13 objects (Pod 8, Deployment 2, Job 2, Service 1), 7 unhealthy
unhealthy Pod shop/api-badimage: ImagePullBackOff, 0/1 ready; api waiting ImagePullBackOff
...
unhealthy Pod shop/migrate-dvl69: Error, 0/1 ready; migrate terminated Error exit 3
unhealthy Pod shop/migrate-gtc4v: Error, 0/1 ready; migrate terminated Error exit 3
unhealthy Pod shop/worker-56bff96fc9-c8xj6: CrashLoopBackOff, 0/1 ready, 3 restarts; worker waiting CrashLoopBackOff, last Error exit 1
unhealthy Deployment shop/worker: 0/1 ready, 1 updated, 0 available
unhealthy Job shop/migrate: Failed, 0/1 succeeded, 2 failed, BackoffLimitExceeded; Job has reached the specified backoff limit
Pod shop/seed-lxmkd: Completed, 0/1 ready
...
Deployment shop/web: 2/2 ready, 2 updated, 2 available
Job shop/seed: Complete, 1/1 succeeded
Service shop/web: ClusterIP, 10.96.114.207, 80->8080/TCP

$ squeeze kubectl --max-events 2 < internal/formats/kubectl/testdata/events/input.json
kubectl: 48 events, 6 warnings
warning Pod shop/worker-56bff96fc9-c8xj6 BackOff: Back-off restarting failed container worker in pod worker-56bff96fc9-c8xj6_shop(d39d6608-c90e-4a54-9e04-1b09136cbb47) (x7)
warning Pod shop/api-badimage Failed: Error: ImagePullBackOff (x4)
... 4 more warning events: Failed 2, BackoffLimitExceeded 1, FailedScheduling 1 (--max-events 0)
```

Each line is `[unhealthy] <Kind> <namespace>/<name>: <status and counts>`, then `; ` and one entry per container that is not running and ready or has restarted, then `; ` and the message of the condition that explains the state. A cluster-scoped object prints its name alone. Objects keep the input's order inside the unhealthy and healthy groups.

| Kind | Printed | Unhealthy when |
|---|---|---|
| `Pod` | the `STATUS` that `kubectl get pods` prints, computed the same way (`Init:<reason>`, `Init:N/M`, the last container's waiting or terminated reason, `Signal:N`, `ExitCode:N`, `NotReady`, `Terminating`, `Unknown`, `SchedulingGated`), ready containers, restarts; per container `waiting <reason>`, `terminated <reason> exit N`, `running, not ready` and `last <reason> exit N`; the pod's `status.message` or the `PodScheduled=False` message | the phase is `Pending`, `Failed` or `Unknown`, or the phase is `Running` and a container is not ready |
| `Deployment` | ready/desired, updated, available | ready, updated or available is below `spec.replicas`, `Progressing` is `False` or `ReplicaFailure` is `True` |
| `StatefulSet` | ready/desired, updated | ready is below `spec.replicas` |
| `ReplicaSet`, `ReplicationController` | ready/desired, current | ready is below `spec.replicas`, or `ReplicaFailure` is `True` |
| `DaemonSet` | ready/desired, updated, available, misscheduled | ready or available is below `desiredNumberScheduled`, or a pod is misscheduled |
| `Job` | the `STATUS` of `kubectl get jobs` (`Complete`, `Failed`, `Terminating`, `Suspended`, `FailureTarget`, `Running`), succeeded/completions, failed, active, the reason of the failing condition | `Failed` or `FailureTarget` |
| `CronJob` | `Suspended`, active jobs, last schedule time | never |
| `Node` | `Ready`, `NotReady` or `Unknown` and `SchedulingDisabled`, as `kubectl get nodes` prints them; every other condition that is `True` (`MemoryPressure`, `DiskPressure`, `PIDPressure`, `NetworkUnavailable`); the kubelet version | `Ready` is not `True`, or another condition is `True` |
| `PersistentVolumeClaim`, `PersistentVolume` | the phase, or `Terminating` | a claim that is not `Bound`; a volume that is `Failed` |
| `Service` | type, cluster IP or external name, ports as `port->targetPort:nodePort/protocol`, `external IP pending` for a `LoadBalancer` without an ingress | never |
| `Secret` | type and data keys | never |
| `ConfigMap` | data keys | never |
| any other kind, custom resources included | `status.phase` and a `Ready` or `Available` condition that is `False` | the phase is `Failed`, or `Ready` or `Available` is `False` |

- Pod, Job and Node statuses follow `printPod`, `printJob` and `printNode` in Kubernetes `pkg/printers/internalversion/printers.go` at v1.33.1, the code behind `kubectl get`'s tables. The ready, updated and available columns follow `printDeployment`, `printStatefulSet`, `printReplicaSet` and `printDaemonSet` in the same file.
- A missing `spec.replicas` counts as 1, the API's default.
- An object of a known kind whose `spec` or `status` does not have the API's shape prints as `unhealthy ... unreadable status`, so a broken object is not counted healthy.
- An object with `metadata.deletionTimestamp` shows `Terminating` or `terminating`.
- `Event` objects are not listed as objects. The header counts them and the Warning ones, and each Warning event prints as `warning <Kind> <namespace>/<name> <reason>: <message> (xN)`, newest first by `series.lastObservedTime`, `lastTimestamp`, `deprecatedLastTimestamp`, `eventTime` or `firstTimestamp`, compared as RFC 3339 times, so `00:00:01.5Z` is newer than `00:00:01Z`; an event whose time does not parse sorts after the ones that do. Warning events with the same object, reason and message print once with their counts summed. Both `v1` events (`involvedObject`, `message`, `count`) and `events.k8s.io/v1` events (`regarding`, `note`, `deprecatedCount`, `series.count`) are read. `Normal` events are counted only. An event whose fields do not have the API's shape, such as a `count` that is a string, prints as `warning Event <namespace>/<name> unreadable event` and counts as a Warning event, so a broken event is not dropped.
- Past `--max-objects`, the notice counts the cut objects, the unhealthy ones among them and their kinds: `... 27 more objects, 3 unhealthy: Pod 12, Deployment 4, ReplicaSet 4, Service 3, DaemonSet 2, 1 other kind (--max-objects 0)`. Past `--max-events`, it counts the cut warning events by reason.
- Control characters and terminal escape sequences are removed from every name, status, reason and message, in the text and in `--output json`; a newline or tab becomes a space. The `escapes` fixture shows it.
- Messages (a condition message, `status.message`, an event message) are cut at 200 bytes with `...` in the text; `--output json` keeps them whole.
- A line after the JSON document that reads as an error or a warning, such as `error: the server doesn't have a resource type "widgets"` that `kubectl get pods,widgets -o json 2>&1` prints after the pods, prints after the header as `error: <line>` or `warning: <line>`, without an `error:` or `Warning:` prefix; an error line sets `error` and a warning line sets `warning`. Error and warning lines before the document, such as a `Warning:` line about a deprecated API, are read the same way; detection skips them, so a Secret behind such a line still goes through this format and not `generic`. Other lines after the document are dropped. The `trailing-errors` and `leading-warning` fixtures show it.

### Secrets

squeeze never reads or prints a value that can hold a secret:

- `data` and `stringData` of a Secret and `data` and `binaryData` of a ConfigMap: only the keys are printed, sorted.
- Every annotation value of every object. `kubectl apply` stores the whole applied manifest in the `kubectl.kubernetes.io/last-applied-configuration` annotation, so for a Secret it holds the values in plain text (https://kubernetes.io/docs/tasks/manage-kubernetes-objects/declarative-config/).
- Labels, the pod spec and pod templates, so container `env` values, commands and arguments.
- A container's `terminated.message`, which holds what the container wrote to its termination log, and `waiting.message`.

A test extracts every Secret and ConfigMap value, base64-decoded too, every annotation value, the values inside a `last-applied-configuration` and every `env` value from the fixtures and asserts that none reaches the text or `--output json`. Another test changes all of them and asserts that `data` does not change. The fuzz target checks the same on random input.

## Flags

| Flag | Default | Effect |
|---|---|---|
| `--max-objects` | `50` | objects listed, unhealthy first, `0` for all |
| `--max-events` | `20` | warning events listed, `0` for all |

## Conditions for `--fail-on`

| Condition | Present when |
|---|---|
| `error` | an error line follows the JSON |
| `fail` | an object is unhealthy, as the table above defines it |
| `restarts` | a pod's containers restarted at least once |
| `warning` | the input holds an event of type `Warning`, or a warning line follows the JSON |

## JSON data

`data` holds `tool`, `tool_version` (empty: the JSON carries no kubectl version), `objects_total`, `unhealthy`, `restarts` (the sum over pods), `kinds` (`name`, `count`, most objects first), `objects`, `objects_omitted`, `unhealthy_omitted`, `omitted_by_kind`, `events_total`, `warnings` (Warning events in the input), `events` (the listed warning events), `events_omitted`, `omitted_by_reason` and, when lines before or after the JSON read as errors or warnings, `diagnostics` (`severity`, `summary`, `count`). Each object has `kind`, `namespace`, `name`, `unhealthy`, `status`, `details`, `restarts` and, when set, `containers`, `keys` and `message`. Each event has `kind`, `namespace` and `name` of the object it is about, `reason`, `message`, `count` and `last`.

## Limitations

- Only `-o json` is read. `-o yaml`, `-o wide` and kubectl's default tables go to `generic`, which does not know which values are secret: pipe Secrets through `squeeze` only as JSON.
- Event messages are printed as Kubernetes wrote them; a controller that puts a secret in an event message would leak it the same way `kubectl get events` does.
- Health is per object. squeeze does not join objects, so a Deployment line does not name its failing pods; list the pods in the same call (`kubectl get deploy,pods -o json`).
- A custom resource is judged by its `Ready` or `Available` condition and `status.phase` only; any other convention prints the object without a health verdict.
- `kubectl get` errors (an unknown resource type, a forbidden namespace) and warnings go to stderr. squeeze reads them only when stderr is piped too (`2>&1`). A line before the JSON that does not read as an error or a warning makes the input fail detection, and it goes to `generic`, which does not know which values are secret.
