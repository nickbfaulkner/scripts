
import boto3
import sys

if (len(sys.argv) != 3):
    print('whoami.py <access_key_id> <secret_access_key_id>')
    sys.exit(1)

access_key_id = sys.argv[1]
secret_access_key_id = sys.argv[2]

identity = boto3.client('sts', aws_access_key_id=access_key_id, aws_secret_access_key=secret_access_key_id).get_caller_identity()

print('Account: ' + identity['Account'])
print('User: ' + identity['Arn'])
