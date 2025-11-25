# Settings
set dotenv-load

# Variables
regions := "us-east-1 us-east-2 us-west-1 us-west-2 ap-south-1 ap-northeast-1 ap-northeast-2 ap-northeast-3 ap-southeast-1 ap-southeast-2 ca-central-1 eu-central-1 eu-west-1 eu-west-2 eu-west-3 eu-north-1 eu-south-1 sa-east-1"
region := env('AWS_REGION', 'us-east-1')
bucket := env('RUNVOY_RELEASES_BUCKET', 'runvoy-releases-us-east-1')
axiom_bucket := env('AXIOM_LOG_FORWARDER_BUCKET', 'axiom-forwarder')
axiom_token := env('AXIOM_TOKEN', '')
axiom_dataset := env('AXIOM_DATASET', 'runvoy-dev')
orchestrator_log_group := env('ORCHESTRATOR_LOG_GROUP', '/aws/lambda/runvoy-orchestrator')
event_processor_log_group := env('EVENT_PROCESSOR_LOG_GROUP', '/aws/lambda/runvoy-event-processor')
stack_name := env('RUNVOY_CLOUDFORMATION_BACKEND_STACK', 'runvoy-backend')
admin_email := env('RUNVOY_ADMIN_EMAIL', 'admin@runvoy.site')
version := trim(read('VERSION'))
git_short_hash := trim(`git rev-parse --short HEAD`)
build_date := datetime_utc('%Y%m%d')
build_flags_x := '-X=runvoy/internal/constants.version='
build_flags_regions := '-X=runvoy/internal/providers/aws/constants.rawReleaseRegions='
build_version := version + '-' + build_date + '-' + git_short_hash
regions_comma := replace(regions, ' ', ',') # Convert space-separated regions to comma-separated for ldflags (spaces cause issues)
replication_target_regions := replace(replace(regions_comma, ',us-east-1', ''), 'us-east-1,', '') # Replication target regions: all regions except us-east-1 (the primary region)
build_flags := build_flags_x + build_version + ' ' + build_flags_regions + regions_comma

# Import subfiles
import '.justfiles/build.just'
import '.justfiles/deploy.just'
import '.justfiles/test.just'
import '.justfiles/dev.just'
import '.justfiles/lint.just'
import '.justfiles/infra.just'
import '.justfiles/release.just'
import '.justfiles/utils.just'

# Aliases
alias r := runvoy

## Commands
# Build the CLI binary and run it with the given arguments
[default]
runvoy *ARGS: build-cli
    ./bin/runvoy --verbose {{ARGS}}
