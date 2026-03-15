# Stack using a raw git source
resource "zenfra_stack" "app" {
  name     = "Application Stack"
  space_id = zenfra_space.production.id

  iac {
    engine  = "terraform"
    version = "1.9.0"
  }

  source {
    type = "raw_git"
    raw_git {
      url = "https://github.com/example/infra.git"
      ref {
        type = "branch"
        name = "main"
      }
      path = "stacks/app"
    }
  }

  triggers {
    on_push {
      enabled = true
      paths   = ["stacks/app/**"]
    }
  }
}

# Stack using a VCS integration
resource "zenfra_stack" "network" {
  name           = "Network Stack"
  space_id       = zenfra_space.production.id
  worker_pool_id = zenfra_worker_pool.private.id

  iac {
    engine  = "opentofu"
    version = "1.8.0"
  }

  source {
    type = "vcs"
    vcs {
      provider       = "github"
      integration_id = data.zenfra_vcs_integration.github.id
      repository_id  = "example/network-infra"
      ref {
        type = "branch"
        name = "main"
      }
    }
  }
}
