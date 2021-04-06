
Helm-service
===========

Helm Chart for the keptn-contrib dynatrace-sli-service


## Configuration

The following table lists the configurable parameters of the dynatrace-sli-service chart and their default values.

| Parameter                | Description             | Default        |
| ------------------------ | ----------------------- | -------------- |
| `dynatraceSliService.image.repository` | Container image name | `"docker.io/keptncontrib/dynatrace-sli-service"` |
| `dynatraceSliService.image.pullPolicy` | Kubernetes image pull policy | `"IfNotPresent"` |
| `dynatraceSliService.image.tag` | Container tag | `""` |
| `dynatraceSliService.service.enabled` | Creates a kubernetes service for the dynatrace-sli-service | `true` |
| `distributor.stageFilter` | Sets the stage this dynatrace-sli-service belongs to | `""` |
| `distributor.serviceFilter` | Sets the service this dynatrace-sli-service belongs to | `""` |
| `distributor.projectFilter` | Sets the project this dynatrace-sli-service belongs to | `""` |
| `distributor.image.repository` | Container image name | `"docker.io/keptn/distributor"` |
| `distributor.image.pullPolicy` | Kubernetes image pull policy | `"IfNotPresent"` |
| `distributor.image.tag` | Container tag | `""` |
| `remoteControlPlane.enabled` | Enables remote execution plane mode | `false` |
| `remoteControlPlane.api.protocol` | Used protocol (http, https | `"https"` |
| `remoteControlPlane.api.hostname` | Hostname of the control plane cluster (and port) | `""` |
| `remoteControlPlane.api.apiValidateTls` | Defines if the control plane certificate should be validated | `true` |
| `remoteControlPlane.api.token` | Keptn api token | `""` |
| `imagePullSecrets` | Secrets to use for container registry credentials | `[]` |
| `serviceAccount.create` | Enables the service account creation | `true` |
| `serviceAccount.annotations` | Annotations to add to the service account | `{}` |
| `serviceAccount.name` | The name of the service account to use. | `""` |
| `podAnnotations` | Annotations to add to the created pods | `{}` |
| `podSecurityContext` | Set the pod security context (e.g. fsgroups) | `{}` |
| `securityContext` | Set the security context (e.g. runasuser) | `{}` |
| `resources` | Resource limits and requests | `{}` |
| `nodeSelector` | Node selector configuration | `{}` |
| `tolerations` | Tolerations for the pods | `[]` |
| `affinity` | Affinity rules | `{}` |





