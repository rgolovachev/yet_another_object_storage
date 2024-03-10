# yet another object storage

Как и в обычном object storage, файлы хранятся в бакетах.
Мой object storage умеет хранить произвольные файлы, побитые на несколько чанков, на нескольких шардах, тем самым балансируя нагрузку.
По дефолту в `docker-compose.yml` выставлено три шарда. Кроме того есть API сервис, который реализует
REST API и предоставляет возможность работать с файлами (по дефолту он один, но возможно поставить их
сразу несколько и ничего не должно сломаться). Аналогичная ситуация с metadata сервисом, который хранит
метаданные хранящихся файлов. Ну и еще я реализовал сервис статистики, который собирает данные о загруженности
шардов. К нему тоже имеется возможность обратиться `curl`-ом по http.

## Как запустить и работать с этим

`docker compose build && docker compose up` в одном терминале, а в другом `curl`-ом посылать запросы к API
сервису или сервису статистики.

## Важное замечание

Нужно немного подождать, пока все сервисы поднимутся в докере. Это занимает в районе 30-40 секунд. В логах
docker-compose будет видно, когда сервис поднялся (будет сообщения типа `... is started`). Такая задержка в основном из-за
того, что каждый раз приходится подкачивать гошные библиотеки.

## Как работать с API сервисом

Если что, я считаю что API сервис живет на 18100 порту (так указано в `docker-compose.yml`). Но он так то может
быть любым.

`curl -X POST 0.0.0.0:18100/my_bucket` - создать бакет

`curl -X GET 0.0.0.0:18100/my_bucket` - посмотреть какие файлы лежат в бакете

`curl -X DELETE 0.0.0.0:18100/my_bucket` - удалить бакет (но для начала надо удалить все файлы из него)

`curl -X POST 0.0.0.0:18100/my_bucket/my_file.txt -d "hello"` - создать файл в бакете

`curl -X GET 0.0.0.0:18100/my_bucket/my_file.txt` - получить файл из бакета

`curl -X DELETE 0.0.0.0:18100/my_bucket/my_file.txt` - удалить файл из бакета

## Как работать с сервисом статистики

По дефолту сервис статистики живет на порту 37373

`curl -X GET 0.0.0.0:37373/stat/shard/<shard_name>` - получить статистику по чанкам на шарде `<shard_name>`, где `<shard_name>` - текстовое название шарда из файла `config.json`

## Что за `config.json`

В конфиге хранятся порты всех трех (API, metadata, statistics) сервисов + названия шардов и их порты. Этот
файлик разумеется можно менять, но нужно отразить изменения в `docker-compose.yml`, чтобы все заработало.

