#!/bin/bash
if [[ "pull_request" == "push" && ("refs/pull/52/merge" == "refs/heads/ben" || "refs/pull/52/merge" == "refs/heads/aj") ]]; then echo "test"; fi
