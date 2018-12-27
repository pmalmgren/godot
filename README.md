# Godot

Turns a dotfile repository into a Docker development environment.

## Usage

Here's what you need to get started:

  - A GitHub repository with your dotfiles
  - A `README.md` document that describes your desired development environment
  - Docker

```
$ godot build --image-tag dev-env github.com/pmalmgren/godot
$ docker run --rm -it godot-development-environment
dev-shell$
```

## Configuration

`godot` uses a section in your project's `README.md` to build a Docker image. A comprehensive example is given below.

## godot configuration

`godot` configuration starts with a second level heading named `godot configuration`. `godot` will ignore anything in the top section, so feel free to add any documentation here.

```
username: godot
dotfile-directory: dotfiles
entrypoint: zsh

packages:
  - neovim
  - git
  - tmux
  - zsh
  - curl

# system-setup runs as root, define volumes etc.
system-setup:
  - VOLUME /home/$username/.ssh
  - VOLUME /home/$username/.credentials/
  - VOLUME /home/$username/persistent/
  - RUN chsh -s /usr/bin/zsh $username

# user-setup runs as the user defined above in username.
user-setup:
  - RUN curl -L http://install.ohmyz.sh | sh || true
  - RUN curl -fLo ~/.local/share/nvim/site/autoload/plug.vim --create-dirs https://raw.githubusercontent.com/junegunn/vim-plug/master/plug.vim
  - RUN git clone https://github.com/zsh-users/zsh-syntax-highlighting.git /home/$username/.oh-my-zsh/custom/plugins/zsh-syntax-highlighting
  - RUN rm .zshrc || true
```
