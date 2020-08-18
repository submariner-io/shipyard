#!/bin/bash
set -e

gitlint --commits origin/master..HEAD
