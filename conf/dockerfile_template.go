package conf

const dockerfileTemplate = `FROM debian:stretch-slim

MAINTAINER Godot

ARG username={{.Username}}

# System setup

RUN \
  apt-get update && \
  apt-get -y upgrade && \
  apt-get clean && \
  apt-get -y install curl && \
  apt-get -y install stow && \
  apt-get -y install make && \
  apt-get -y install locales
{{if .Packages}}
RUN apt-get -y install {{range $element := .Packages}}{{printf "%s " $element}}{{end}}
{{end}}
# locale

RUN \
  echo "LC_ALL=en_US.UTF-8" >> /etc/environment && \
  echo "en_US.UTF-8 UTF-8" >> /etc/locale.gen && \
  echo "LANG=en_US.UTF-8" > /etc/locale.conf && \
  locale-gen en_US.UTF-8

# Create the user and copy over files

RUN useradd -ms /bin/bash $username

{{range $element := .SystemSetup}}{{printf "%s\n" $element}}{{end}}
ADD {{.DotfileDirectory}}/ /home/$username/dotfiles/

USER $username
WORKDIR /home/$username/

{{range $element := .UserSetup}}{{printf "%s\n" $element}}{{end}}
WORKDIR /home/$username/dotfiles/
RUN ls -la | grep ^d | awk '{ print $9 }' | grep -v '^\.\+$' | xargs stow

USER $username
WORKDIR /home/$username

CMD ["{{.EntryPoint}}"]`
