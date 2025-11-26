#!/bin/bash
git filter-branch -f --msg-filter '
  sed "/🤖 Generated with \[Claude Code\]/d; /Co-Authored-By: Claude/d"
' -- --all
