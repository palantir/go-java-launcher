[![CircleCI Build Status](https://circleci.com/gh/palantir/go-java-launcher/tree/develop.svg?style=shield)](https://circleci.com/gh/palantir/go-java-launcher)
[![Download](https://api.bintray.com/packages/palantir/releases/go-java-launcher/images/download.svg) ](https://bintray.com/palantir/releases/go-java-launcher/_latestVersion)

# go-java-launcher

A simple Go program for launching Java programs from a fixed configuration. This program replaces Gradle-generated Bash
launch scripts which are susceptible to attacks via injection of environment variables of the form `JAVA_OPTS='$(rm -rf
/)'`.

The launcher accepts as configuration two YAML files as follows:

```yaml
# StaticLauncherConfig
# The type of configuration, must be the string "java"
configType: java
# The version of the configuration format, must be the integer 1
configVersion: 1
# The main class to be run
mainClass: my.package.Main
# Path to the JRE, defaults to the JAVA_HOME environment variable if unset
javaHome: javaHome
# The classpath entries; the final classpath is the ':'-concatenated list in the given order
classpath:
  - ./foo.jar
# Environment Variables to be set in the environment
env:
  CUSTOM_VAR: CUSTOM_VALUE
# JVM options to be passed to the java command
jvmOpts:
  - '-Xmx1g'
# Arguments passed to the main method of the main class
args:
  - arg1
```

```yaml
# CustomLauncherConfig
configType: java
configVersion: 1
# Environment variables to be set in the runtime environment
env:
  CUSTOM_VAR: CUSTOM_VALUE
  CUSTOM_PATH: '{{CWD}}/some/path'
# JVM options to be passed to the java command
jvmOpts:
  - '-Xmx2g'
```

The launcher is invoked as:
```
go-java-launcher [<path to StaticLauncherConfig> [<path to CustomLauncherConfig>]]
```

where the
static configuration file defaults to `./launcher-static.yml` and the custom configuration file defaults to
`./launcher-custom.yml`. It assembles the configuration options and executes the following command (where `<static.xyz>`
and `<custom.xyz>` refer to the options from the two configuration files, respectively):

```
<javaHome>/bin/java \
  <static.jvmOpts> \
  <custom.jvmOpts> \
  -classpath <classpath entries> \
  <static.mainClass> \
  <static.args>
```

Note that the custom `jvmOpts` appear after the static `jvmOpts` and thus typically take precendence; the exact
behaviour may depend on the Java distribution.

`env` block, both in static and custom configuration, supports restricted set of automatic expansions for values
assigned to environment variables. Variables are expanded if they are surrounded with `{{` and `}}` as shown above
for `CUSTOM_PATH`. The following fixed expansions are supported:

* `{{CWD}}`: The current working directory of the user which executed this process

Expansions are only performed on the values. No expansions are performed on the keys. Note that the JAVA_HOME
environment cannot be overwritten with this mechanism; use the `javaHome` mechanism in `StaticLauncherConfig` instead.

# License
This repository is made available under the [Apache 2.0 License](http://www.apache.org/licenses/LICENSE-2.0).
