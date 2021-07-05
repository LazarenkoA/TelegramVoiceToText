# Voice 2 Text
Клиент для телеграмма на базе [gotd/td](https://github.com/gotd/td), задача данного клиента конвертировать голосовые сообщения в текст, конвертация осуществляется через
[yandex speechkit](https://cloud.yandex.ru/docs/speechkit/stt/). Прелесть данной реализации в том, что конвертация происходит в личной переписке, ведь сторонних ботов нельзя использовать в личной переписке.

Для использования, скачеваем последний [релиз](https://github.com/LazarenkoA/TelegramVoiceToText/releases), устанавливаем нужные переменные окружения, запускаем, следуем подсказкам.
Переменные окружения:

**yandex** *(получаются в [консоли управления](https://console.cloud.yandex.ru))*
- KEY - *статический ключ доступа сервисного аккаунта*
- IDAPIKEY - *идентификатор статичесого ключа доступа*
- APIKEY - *API key сервисного аккаунта*
- BUCKET - *бакет*


**телеграм** *(получаются [тут](https://my.telegram.org/auth))*
- APPID
- APPHASH  