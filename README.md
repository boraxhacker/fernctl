# fernctl

fernctrl implements simple commands and is designed for use with home-fern.

## Examples

```shell
fernctl ssm get /some/parameter
fernctl ssm get path:/some/path
fernctl ssm delete /some/parameter
fernctl ssm delete path:/some/path
```
### Sync

The sync method imports a yaml file in the following format into SSM under the prefix provided. 

**BEWARE** sync prunes the prefix: deletes parameter keys not included in the file.

```shell
sops -d somefile.enc.yaml > somefile.yaml

fernctl ssm sync prefix somefile.yaml

rm somefile.yaml
```

### Sample yaml

```yaml
dockerhub:
  username: myusername
  password: mypassword
postgres:
  admin:
    username: pguser
    password: pgpassword
```

### SSM Results

```shell
fernctl ssm sync prefix tests/sample.yaml

Upserting /prefix/dockerhub/username
Upserting /prefix/dockerhub/password
Upserting /prefix/postgres/admin/username
Upserting /prefix/postgres/admin/password

fernctl ssm get path:/prefix

/prefix/dockerhub/password: 'mypassword'
/prefix/dockerhub/username: 'myusername'
/prefix/postgres/admin/password: 'pgpassword'
/prefix/postgres/admin/username: 'pguser'

# cleanup
fernctl ssm delete path:/prefix
Success: [/prefix/dockerhub/password /prefix/dockerhub/username /prefix/postgres/admin/password /prefix/postgres/admin/username]
Failures: []
```

