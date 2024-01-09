<p align="right">
<a href="https://autorelease.general.dmz.palantir.tech/palantir/go-java-launcher"><img src="https://img.shields.io/badge/Perform%20an-Autorelease-success.svg" alt="Autorelease"></a>
</p>

[![CircleCI Build Status](https://circleci.com/gh/palantir/go-java-launcher/tree/develop.svg?style=shield)](https://circleci.com/gh/palantir/go-java-launcher)
[![Download](https://api.bintray.com/packages/palantir/releases/go-java-launcher/images/download.svg) ](https://bintray.com/palantir/releases/go-java-launcher/_latestVersion)

# go-java-launcher

A simple Go program for launching processes from a fixed configuration. This program replaces Gradle-generated Bash
launch scripts which are susceptible to attacks via injection of environment variables of the form `JAVA_OPTS='$(rm -rf
/)'`.

The launcher accepts as configuration two YAML files as follows:

### launcher-static.yml

```yaml
# StaticLauncherConfig - java version
# REQUIRED - The type of configuration, must be the string "java"
configType: java
# REQUIRED - The version of the configuration format, must be the integer 1
configVersion: 1
# REQUIRED - The main class to be run
mainClass: my.package.Main
# OPTIONAL - Path to the JRE or environment variable name (e.g. $JAVA_11_HOME). Defaults to the JAVA_HOME environment variable if unset,
#  or if the specified environment variable is not set at runtime.
javaHome: /opt/palantir/jdk8/Contents/Home
# REQUIRED - The classpath entries; the final classpath is the ':'-concatenated list in the given order
classpath:
  - ./foo.jar
# OPTIONAL - Environment Variables to be set in the environment (Note: cannot be referenced on args list)
env:
  CUSTOM_VAR: CUSTOM_VALUE
# OPTIONAL - JVM options to be passed to the java command
jvmOpts:
  - '-Xmx1g'
# OPTIONAL - Arguments passed to the main method of the main class
args:
  - arg1
# OPTIONAL - A list of directories to be created before executing the command. Must be relative to CWD and over [A-Za-z0-9].
dirs:
  - var/data/tmp
  - var/log
# OPTIONAL - A map of configurations of subProcesses to launch
subProcesses:
  SUB_PROCESS_NAME:
    # another StaticLauncherConfig though it cannot have its own subProcesses, and uses its parent's configVersion
    configType: executable
    env:
      CUSTOM_VAR: CUSTOM_VALUE
    executable: "{{CWD}}/service/lib/envoy/envoy"
    dirs:
      - var/data/tmp
      - var/log
```

```yaml
# StaticLauncherConfig - executable version
# REQUIRED - The type of configuration, must be the string "executable"
configType: executable
# REQUIRED - The version of the configuration format, must be the integer 1
configVersion: 1
# OPTIONAL - Environment Variables to be set in the environment (Note: cannot be referenced on args list)
env:
  CUSTOM_VAR: CUSTOM_VALUE
# REQUIRED - The full path to the executable file, limited to whitelisted values (java, postgres, influxd, grafana-server)
executable: "{{CWD}}/service/bin/postgres"
# OPTIONAL - Arguments passed to the main method of the excutable or main class
args:
  - arg1
# OPTIONAL - A list of directories to be created before executing the command. Must be relative to CWD and over [A-Za-z0-9].
dirs:
  - var/data/tmp
  - var/log
# OPTIONAL - A map of configurations of secondary processes to launch
subProcesses:
  SUB_PROCESS_NAME:
    # another StaticLauncherConfig though it cannot have its own subProcesses, and uses its parent's configVersion
    configType: executable
    env:
      CUSTOM_VAR: CUSTOM_VALUE
    executable: "{{CWD}}/service/lib/envoy/envoy"
    dirs:
      - var/data/tmp
      - var/log
```

### launcher-custom.yml

```yaml
# CustomLauncherConfig
# REQUIRED - The type of configuration, must be the string "java" or "executable"
configType: java
# REQUIRED - The version of the configuration format, must be the integer 1
configVersion: 1
# OPTIONAL - Environment Variables to be set in the environment, will override defaults in static config (Note: cannot be referenced on args list)
env:
  CUSTOM_VAR: CUSTOM_VALUE
  CUSTOM_PATH: '{{CWD}}/some/path'
# Additional JVM options to be passed to the java command, will override defaults in static config. Ignored if configType is "executable"
jvmOpts:
  - '-Xmx2g'
# OPTIONAL - A map of configurations of secondary processes to launch
subProcess:
  SUB_PROCESS_NAME:
    # another CustomLauncherConfig though it cannot have its own subProcesses, and uses its parent's configVersion
    configType: executable
    env:
      CUSTOM_VAR: CUSTOM_VALUE
```

The launcher is invoked as:
```
go-java-launcher [<path to StaticLauncherConfig> [<path to CustomLauncherConfig>]]
```

where the static configuration file defaults to `./launcher-static.yml` and the custom configuration file defaults to
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

If any subProcesses are defined, they will be launched as child processes of the main process, with all of these
processes occupying their own process group. Additionally, a monitor subProcess will be launched, which terminates
the group, should the main process die.

`env` block, both in static and custom configuration, supports restricted set of automatic expansions for values
assigned to environment variables. Variables are expanded if they are surrounded with `{{` and `}}` as shown above
for `CUSTOM_PATH`. The following fixed expansions are supported:

* `{{CWD}}`: The current working directory of the user which executed this process

Expansions are only performed on the values. No expansions are performed on the keys. Note that the JAVA_HOME
environment cannot be overwritten with this mechanism; use the `javaHome` mechanism in `StaticLauncherConfig` instead.

All output from `go-java-launcher` itself, and from the launch of all processes themselves is directed to stdout.

## Java heap and container support

By _default_, when starting a java process inside a container (as indicated by the presence of ``CONTAINER`` env
variable):
1. The `-XX:ActiveProcessorCount` is unset, it will remain unset.
1. Args with prefix``-Xmx|-Xms`` in both static and custom jvm opts will be filtered out. If neither
   ``-XX:MaxRAMPercentage=`` nor ``-XX:InitialRAMPercentage=`` prefixes are present in either static or custom jvm opts
   ``-Xmx|-Xms`` will both be set to be 75% of the cgroups memory limit minus 3mb per processor, with a minimum value of
   50% of the heap.

This will cause the JVM 11+ to discover the ``MaxRAM`` value using Linux cgroups, and calculate the heap sizes as the specified
percentage of ``MaxRAM`` value, e.g. ``max-heap-size = MaxRAM * MaxRamPercentage``.

If the experimental flag `containerV2` is set to `false`, the behavior will be as follows:
1. If `-XX:ActiveProcessorCount` is unset, it will be set based on the discovered cgroup configurations and host
   information to a value between 2 and the number of processors reported by the runtime. You can read more about the
   reasoning behind this [here](https://github.com/palantir/go-java-launcher/issues/313).
1. Args with prefix``-Xmx|-Xms`` in both static and custom jvm opts will be filtered out.
1. If neither ``-XX:MaxRAMPercentage=`` nor ``-XX:InitialRAMPercentage=`` prefixes are present in either static or
   custom jvm opts, both will be set to ``75.0`` (i.e. ``-XX:InitialRAMPercentage=75.0 -XX:MaxRAMPercentage=75.0 `` will
   be appended after all the other jvm opts).

### Overriding default values

Developers can override the heap percentage in containers by specifying both ``-XX:MaxRAMPercentage=``
and ``-XX:InitialRAMPercentage=`` and we recommend setting both to the same value.

Developers can specify both ``MaxRAMPercentage|InitialRAMPercentage``
together with ``-Xmx|-Xms`` overrides safely: ``-Xmx/-Xms`` overrides ALWAYS take precedence and will be filtered out
when running inside a container, as per logic detailed above.

### Disabling container support

This behavior can be disabled by setting the following in ``launcher-custom.yml``:

```yaml
configType: java
...
dangerousDisableContainerSupport: true
```

Alternatively, the presence of ``-XX:MaxRAM=`` prefix in either static or custom jvm opts will also disable this
behavior.

# go-init

This repository also publishes a binary called `go-init` that supports the commands `start`, `status`, and `stop`, in
adherence with the
[Linux Standard Base](http://refspecs.linuxbase.org/LSB_3.1.1/LSB-Core-generic/LSB-Core-generic/iniscrptact.html)
specification for init scripts. The binary reads configuration from (relative to its working directory)
`service/bin/launcher-static.yml` and `var/conf/launcher-custom.yml` in the same vein as `go-java-launcher`, but instead
outputs its own logging and that of the primary process to `var/log/startup.log`.  Logs on the compilation of a
command used to launch a specific subProcesses, and its subsequent stdout and stderr streams are directed to
`var/log/${SUB_PROCESS}-startup.log` files. `go-init` does not launch each `subProcess` as a child process of the
primary process.

Note that while the specification states that the `status` command prints the status of the service, the exact wording
used to denote that status is not defined and subsequently subject to change without warning.

# License
This repository is made available under the [Apache 2.0 License](http://www.apache.org/licenses/LICENSE-2.0).
