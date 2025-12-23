# GCP Deployment

Runvoy provisions GCP resources using Deployment Manager. The CLI applies a
single declarative deployment that creates all backend resources.

## Required APIs

Runvoy enables required APIs via Service Usage during `runvoy infra apply`. If
you prefer to pre-enable them, ensure these are enabled in the target project:

- deploymentmanager.googleapis.com
- run.googleapis.com
- compute.googleapis.com
- vpcaccess.googleapis.com
- firestore.googleapis.com
- pubsub.googleapis.com
- cloudscheduler.googleapis.com
- secretmanager.googleapis.com
- cloudkms.googleapis.com
- artifactregistry.googleapis.com

## Notes

- Project creation and deletion are still handled via Cloud Resource Manager.
- Cloud Run services are only created when images are provided in deploy
  parameters.
