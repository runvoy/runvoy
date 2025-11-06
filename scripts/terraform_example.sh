TF_VAR_injected_variable=testvaluefromlocalenv bin/runvoy \
    run --git-repo https://github.com/runvoy/terraform-example.git \
    --image ubuntu:latest \
    "apt-get update -q > /dev/null && apt-get install -qy curl unzip 2>&1 > /dev/null && curl https://releases.hashicorp.com/terraform/1.13.5/terraform_1.13.5_linux_amd64.zip > a.zip && unzip a.zip && ./terraform apply -auto-approve"