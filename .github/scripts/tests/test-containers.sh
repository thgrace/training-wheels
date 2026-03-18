# Containers: Docker, Docker Compose, Podman

# =============================================================================
section "Containers: Docker"
# =============================================================================
expect_deny 'docker system prune'                      'docker system prune'
expect_deny 'docker system prune -af'                  'docker system prune -af'
expect_deny 'docker volume prune'                      'docker volume prune'
expect_deny 'docker volume prune -f'                   'docker volume prune -f'
expect_deny 'docker network prune'                     'docker network prune'
expect_deny 'docker image prune'                       'docker image prune'
expect_deny 'docker container prune'                   'docker container prune'
expect_deny 'docker rm -f mycontainer'                 'docker rm -f'
expect_deny 'docker rmi -f myimage'                    'docker rmi -f'
expect_deny 'docker volume rm myvolume'                'docker volume rm'
expect_deny 'docker stop $(docker ps -aq)'             'docker stop all containers'

# Docker safe commands
expect_allow 'docker ps'                               'docker ps'
expect_allow 'docker images'                           'docker images'
expect_allow 'docker logs mycontainer'                 'docker logs'
expect_allow 'docker build -t myimage .'               'docker build'
expect_allow 'docker run -it ubuntu bash'              'docker run'
expect_allow 'docker pull nginx:latest'                'docker pull'
expect_allow 'docker inspect mycontainer'              'docker inspect'
expect_allow 'docker stats'                            'docker stats'
expect_allow 'docker exec mycontainer ls'              'docker exec'

# =============================================================================
section "Containers: Docker Compose"
# =============================================================================
expect_deny 'docker-compose down -v'                   'docker-compose down -v'
expect_deny 'docker-compose down --rmi all'            'docker-compose down --rmi all'
expect_deny 'docker-compose rm -v'                     'docker-compose rm -v'
expect_deny 'docker-compose rm -f'                     'docker-compose rm -f'

# =============================================================================
section "Containers: Podman"
# =============================================================================
expect_deny 'podman system prune'                      'podman system prune'
expect_deny 'podman volume prune'                      'podman volume prune'
expect_deny 'podman pod prune'                         'podman pod prune'
expect_deny 'podman image prune'                       'podman image prune'
expect_deny 'podman container prune'                   'podman container prune'
expect_deny 'podman rm -f mycontainer'                 'podman rm -f'
expect_deny 'podman rmi -f myimage'                    'podman rmi -f'
expect_deny 'podman volume rm myvolume'                'podman volume rm'
