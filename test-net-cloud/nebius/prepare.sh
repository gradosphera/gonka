if [ -n "$SSH_KEY_PATH" ]; then
  SSH_KEY_ARG="-i $SSH_KEY_PATH"
else
  SSH_KEY_ARG=""
fi

scp $SSH_KEY_ARG launch.py genesis-overrides.json ubuntu@89.169.111.79:~/
scp $SSH_KEY_ARG launch.py join-1.sh ubuntu@89.169.110.61:~/
scp $SSH_KEY_ARG launch.py join-2.sh ubuntu@89.169.110.250:~/
