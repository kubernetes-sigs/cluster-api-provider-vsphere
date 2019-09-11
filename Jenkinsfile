pipeline {
  agent {
    kubernetes {
      label 'cluster-api-provider-vsphere'
      defaultContainer 'jnlp'
      yamlFile 'JenkinsPod.yaml'
    }
  }

  environment {
    DOCKER_REGISTRY = 'gcr.io'
    ORG        = 'stackpoint-public'
    APP_NAME   = 'cluster-api-provider-vsphere'
    REPOSITORY = "${ORG}/${APP_NAME}"
    GO111MODULE = 'off'
    GOPATH = '/home/jenkins/go'
  }

  stages {

    stage('generate'){
      steps {
        container('golang') {
          dir('/home/jenkins/go/src/github.com/NetApp/cluster-api-provider-vsphere') {
            checkout scm
            sh('go generate ./...')
          }
        }
      }
    }

    stage('build') {
      steps {
        container('builder-base') {
          script {
            image = docker.build("${REPOSITORY}")
          }
        }
      }
    }

    stage('publish: dev') {
      when {
        anyOf {
            branch 'PR-*'
            branch 'netapp-*'
        }
      }
      environment {
        GIT_COMMIT_SHORT = sh(
                script: "printf \$(git rev-parse --short ${GIT_COMMIT})",
                returnStdout: true
        ).trim()
      }
      steps {
        container('builder-base') {
          script {
            docker.withRegistry("https://${DOCKER_REGISTRY}", "gcr:${ORG}") {
              image.push("netapp-dev-${GIT_COMMIT_SHORT}")
            }
          }
        }
      }
    }

    stage('publish: netapp') {
      when {
        branch 'netapp'
      }
      environment {
        GIT_COMMIT_SHORT = sh(
                script: "printf \$(git rev-parse --short ${GIT_COMMIT})",
                returnStdout: true
        ).trim()
      }
      steps {
        container('builder-base') {
          script {
            docker.withRegistry("https://${DOCKER_REGISTRY}", "gcr:${ORG}") {
              image.push("netapp-${GIT_COMMIT_SHORT}")
              image.push("netapp")
            }
          }
        }
      }
    }

  }
}