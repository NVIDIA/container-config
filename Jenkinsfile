/*
# Copyright (c) 2021, NVIDIA CORPORATION.  All rights reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
*/

podTemplate (cloud:'sw-gpu-cloudnative',
    containers: [
    containerTemplate(name: 'docker', image: 'docker:dind', ttyEnabled: true, privileged: true),
    containerTemplate(name: 'golang', image: 'golang:1.16.4', ttyEnabled: true)
  ]) {
    node(POD_LABEL) {
        def scmInfo

        stage('checkout') {
            scmInfo = checkout(scm)
        }

        stage('dependencies') {
            container('golang') {
                sh 'GO111MODULE=off go get -u golang.org/x/lint/golint'
            }
            container('docker') {
                sh 'apk add --no-cache make bash git'
            }
        }

        stage('check') {
            parallel (
                getGolangStages(["assert-fmt", "lint", "vet",])
            )
        }

        stage('unit-test') {
            parallel (
                getGolangStages(["test"])
            )
        }

        def versionInfo
        stage('version') {
            container('docker') {
                versionInfo = getVersionInfo(scmInfo)
                println "versionInfo=${versionInfo}"
            }
        }

        def dist = 'ubuntu20.04'
        def arch = 'amd64'
        def stageLabel = "${dist}-${arch}"

        def imageVersion = versionInfo.version
        def outImage = 'container-toolkit'
        def outImageTag = "${imageVersion}-${dist}"

        stage('build') {
            container('docker') {
                stage (stageLabel) {
                    sh "make IMAGE=${outImage} VERSION=${imageVersion} build-${dist}"
                }
            }
        }

        stage('scan') {
            container('docker') {
                stage(stageLabel) {
                    // TODO: Construct the contents such as ${CONTAMER_SUPPRESS_VULNS:+--suppress-vulns ${CONTAMER_SUPPRESS_VULNS}}
                    def suppress = ''

                    sh "echo cd contamer"
                    sh "echo python3 contamer.py -ls --fail-on-non-os ${suppress}  -- \"${outImage}:${outImageTag}\""
                }
            }
        }

        stage('release') {
            container('docker') {
                stage (stageLabel) {
                    // TODO: Add docker image release
                    def repository = 'sw-gpu-cloudnative-docker-local'

                    def uploadSpec = ""

                    sh "echo starting release with versionInfo=${versionInfo}"
                    if (versionInfo.isTag) {
                        // upload to artifactory repository
                        def server = Artifactory.server 'sw-gpu-artifactory'
                        server.upload spec: uploadSpec
                    } else {
                        sh "echo skipping release for non-tagged build"
                    }
                }
            }
        }
    }
}

def getGolangStages(def targets) {
    stages = [:]

    for (t in targets) {
        stages[t] = getLintClosure(t)
    }

    return stages
}

def getLintClosure(def target) {
    return {
        container('golang') {
            stage(target) {
                sh "make ${target}"
            }
        }
    }
}

// getVersionInfo returns a hash of version info
def getVersionInfo(def scmInfo) {
    def tagged = isTag(scmInfo.GIT_BRANCH)

    def version = tagged ? scmInfo.GIT_BRANCH : getShortSha()


    def versionInfo = [
        version: version,
        isTag: tagged
    ]

    scmInfo.each { k, v -> versionInfo[k] = v }
    return versionInfo
}

def isTag(def branch) {
    if (!branch.startsWith('v')) {
        return false
    }

    def version = shOutput('git describe --all --exact-match --always')
    return version == "tags/${branch}"
}

def getShortSha() {
    return shOutput('git rev-parse --short=8 HEAD')
}

def shOutput(def script) {
    return sh(script: script, returnStdout: true).trim()
}
