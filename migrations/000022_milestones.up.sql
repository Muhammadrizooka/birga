-- Development milestones ("Ko‘nikmalar", Figma 27, 44, 45): what most children can do at an age,
-- in four domains, with optional advice for parents.
CREATE TABLE IF NOT EXISTS milestones (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    domain           TEXT        NOT NULL,
    age_months       SMALLINT    NOT NULL,
    sort_order       INTEGER     NOT NULL DEFAULT 0,
    text_uz          TEXT        NOT NULL,
    text_ru          TEXT        NOT NULL,
    text_en          TEXT        NOT NULL,
    meaning_uz       TEXT        NOT NULL DEFAULT '',
    meaning_ru       TEXT        NOT NULL DEFAULT '',
    meaning_en       TEXT        NOT NULL DEFAULT '',
    how_to_uz        TEXT        NOT NULL DEFAULT '',
    how_to_ru        TEXT        NOT NULL DEFAULT '',
    how_to_en        TEXT        NOT NULL DEFAULT '',
    specialist_uz    TEXT        NOT NULL DEFAULT '',
    specialist_ru    TEXT        NOT NULL DEFAULT '',
    specialist_en    TEXT        NOT NULL DEFAULT '',
    activity_ids     UUID[]      NOT NULL DEFAULT '{}',
    status           TEXT        NOT NULL DEFAULT 'in_review',
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at       TIMESTAMPTZ,

    CONSTRAINT milestones_domain_chk CHECK (domain IN ('communication', 'social', 'thinking', 'movement')),
    CONSTRAINT milestones_age_chk CHECK (age_months BETWEEN 2 AND 72),
    CONSTRAINT milestones_status_chk CHECK (status IN ('published', 'in_review', 'unpublished'))
);

CREATE INDEX IF NOT EXISTS milestones_published_age_idx ON milestones (age_months, domain, sort_order)
    WHERE deleted_at IS NULL AND status = 'published';

-- Milestones a parent ticked for a child ("O‘zlashtirdi"). Shared by the child's parents.
CREATE TABLE IF NOT EXISTS child_milestones (
    child_id     UUID        NOT NULL REFERENCES children (id) ON DELETE CASCADE,
    milestone_id UUID        NOT NULL REFERENCES milestones (id),
    user_id      UUID        REFERENCES users (id) ON DELETE SET NULL,
    achieved_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (child_id, milestone_id)
);

CREATE INDEX IF NOT EXISTS child_milestones_child_achieved_idx ON child_milestones (child_id, achieved_at);

-- Starting list, after the CDC "Learn the Signs. Act Early." milestones (2022), 2 to 5 years.
-- Advice texts are left empty for the content team.
INSERT INTO milestones (domain, age_months, sort_order, text_uz, text_ru, text_en, status) VALUES
('communication', 24, 1, 'Kitobda so‘ragan narsangizni barmog‘i bilan ko‘rsatadi', 'Показывает пальцем в книге то, о чём вы спросили', 'Points to things in a book when you ask', 'published'),
('communication', 24, 2, 'Kamida ikki so‘zni qo‘shib aytadi («yana sut»)', 'Говорит хотя бы два слова вместе («ещё молока»)', 'Says at least two words together, like “more milk”', 'published'),
('communication', 24, 3, 'So‘raganda tanasining kamida ikki qismini ko‘rsatadi', 'Показывает хотя бы две части тела, когда просят', 'Points to at least two body parts when asked', 'published'),
('social', 24, 1, 'Boshqalar xafa bo‘lsa yoki biror joyi og‘risa, buni sezadi', 'Замечает, когда другим больно или грустно', 'Notices when others are hurt or upset', 'published'),
('social', 24, 2, 'Yangi vaziyatda qanday munosabat bildirishni bilish uchun yuzingizga qaraydi', 'В новой ситуации смотрит на ваше лицо, чтобы понять, как реагировать', 'Looks at your face to see how to react in a new situation', 'published'),
('thinking', 24, 1, 'Bir qo‘lida narsani ushlab, ikkinchisi bilan boshqa ish qiladi', 'Держит предмет в одной руке и действует другой', 'Holds something in one hand while using the other hand', 'published'),
('thinking', 24, 2, 'O‘yinchoqdagi tugma, kalit va dastaklarni ishlatib ko‘radi', 'Пробует нажимать кнопки и крутить ручки на игрушке', 'Tries to use switches, knobs or buttons on a toy', 'published'),
('thinking', 24, 3, 'Bir vaqtda bir nechta o‘yinchoq bilan o‘ynaydi', 'Играет с несколькими игрушками одновременно', 'Plays with more than one toy at the same time', 'published'),
('movement', 24, 1, 'To‘pni tepadi', 'Пинает мяч', 'Kicks a ball', 'published'),
('movement', 24, 2, 'Yuguradi', 'Бегает', 'Runs', 'published'),
('movement', 24, 3, 'Qoshiq bilan o‘zi ovqatlanadi', 'Сам ест ложкой', 'Eats with a spoon', 'published'),

('communication', 30, 1, 'Taxminan 50 ta so‘z aytadi', 'Говорит около 50 слов', 'Says about 50 words', 'published'),
('communication', 30, 2, 'Harakat so‘zi bilan ikki va undan ko‘p so‘zni qo‘shib aytadi («kuchuk yugurdi»)', 'Соединяет два и больше слов с глаголом («собачка бежит»)', 'Says two or more words together with one action word, like “doggie run”', 'published'),
('communication', 30, 3, 'Kitobdagi narsani ko‘rsatib so‘rasangiz, nomini aytadi', 'Называет предметы в книге, когда вы показываете и спрашиваете', 'Names things in a book when you point and ask', 'published'),
('social', 30, 1, 'Boshqa bolalar yonida o‘ynaydi, ba’zan ular bilan birga o‘ynaydi', 'Играет рядом с другими детьми, иногда вместе с ними', 'Plays next to other children and sometimes plays with them', 'published'),
('social', 30, 2, '«Menga qara!» deb nima qila olishini ko‘rsatadi', 'Показывает, что умеет: «Смотри!»', 'Shows you what they can do by saying “Look at me!”', 'published'),
('social', 30, 3, 'Aytilganda oddiy tartibga amal qiladi (o‘yinchoqlarni yig‘ishadi)', 'Выполняет простые привычные дела, когда попросят (помогает собрать игрушки)', 'Follows simple routines when told, like helping to pick up toys', 'published'),
('thinking', 30, 1, 'Narsalardan o‘yinda boshqa narsa sifatida foydalanadi (qo‘g‘irchoqqa kubik «yediradi»)', 'Использует предметы понарошку («кормит» куклу кубиком)', 'Uses things to pretend, like feeding a block to a doll as if it were food', 'published'),
('thinking', 30, 2, 'Oddiy muammolarni hal qiladi (narsaga yetish uchun kursichaga chiqadi)', 'Решает простые задачи (встаёт на табурет, чтобы дотянуться)', 'Shows simple problem-solving, like standing on a stool to reach something', 'published'),
('thinking', 30, 3, 'Ikki bosqichli ko‘rsatmani bajaradi («o‘yinchoqni qo‘y va eshikni yop»)', 'Выполняет просьбу из двух шагов («положи игрушку и закрой дверь»)', 'Follows two-step instructions like “put the toy down and close the door”', 'published'),
('movement', 30, 1, 'Ikki oyoqlab sakraydi', 'Прыгает на двух ногах', 'Jumps off the ground with both feet', 'published'),
('movement', 30, 2, 'Ba’zi kiyimlarini o‘zi yechadi', 'Сам снимает некоторые вещи', 'Takes some clothes off by themselves', 'published'),
('movement', 30, 3, 'Kitob varaqlarini bittadan ochadi', 'Переворачивает страницы книги по одной', 'Turns book pages one at a time', 'published'),

('communication', 36, 1, 'Siz bilan kamida ikki marta navbatma-navbat gaplashadi', 'Поддерживает разговор хотя бы из двух реплик с каждой стороны', 'Talks with you in conversation using at least two back-and-forth exchanges', 'published'),
('communication', 36, 2, '«Kim?», «Nima?», «Qayerda?», «Nega?» deb so‘raydi', 'Задаёт вопросы «кто?», «что?», «где?», «почему?»', 'Asks “who”, “what”, “where” or “why” questions', 'published'),
('communication', 36, 3, 'So‘raganda ismini aytadi', 'Называет своё имя, когда спрашивают', 'Says their first name when asked', 'published'),
('social', 36, 1, 'Siz ketganingizdan keyin 10 daqiqa ichida tinchlanadi', 'Успокаивается в течение 10 минут после вашего ухода', 'Calms down within 10 minutes after you leave, like at a childcare drop-off', 'published'),
('social', 36, 2, 'Boshqa bolalarni payqab, ularning o‘yiniga qo‘shiladi', 'Замечает других детей и присоединяется к их игре', 'Notices other children and joins them to play', 'published'),
('thinking', 36, 1, 'Ko‘rsatib bersangiz, doira chizadi', 'Рисует круг, если вы покажете как', 'Draws a circle when you show them how', 'published'),
('thinking', 36, 2, 'Ogohlantirsangiz, issiq narsalarga tegmaydi', 'Не трогает горячее, если вы предупредили', 'Avoids touching hot objects, like a stove, when you warn them', 'published'),
('movement', 36, 1, 'Katta munchoqlarni ipga teradi', 'Нанизывает крупные бусины на шнурок', 'Strings items together, like large beads or macaroni', 'published'),
('movement', 36, 2, 'Ba’zi kiyimlarini o‘zi kiyadi', 'Сам надевает некоторые вещи', 'Puts on some clothes by themselves, like loose pants or a jacket', 'published'),
('movement', 36, 3, 'Vilkadan foydalanadi', 'Пользуется вилкой', 'Uses a fork', 'published'),

('communication', 48, 1, '4 va undan ko‘p so‘zli gaplar tuzadi', 'Говорит предложениями из 4 и более слов', 'Says sentences with four or more words', 'published'),
('communication', 48, 2, 'Qo‘shiq, ertak yoki she’rdan ba’zi so‘zlarni aytadi', 'Повторяет слова из песни, сказки или стишка', 'Says some words from a song, story or nursery rhyme', 'published'),
('communication', 48, 3, 'Kun davomida bo‘lgan kamida bitta voqea haqida gapirib beradi', 'Рассказывает хотя бы об одном событии своего дня', 'Talks about at least one thing that happened during their day', 'published'),
('communication', 48, 4, '«Palto nima uchun kerak?» kabi oddiy savollarga javob beradi', 'Отвечает на простые вопросы вроде «Зачем нужно пальто?»', 'Answers simple questions like “What is a coat for?”', 'published'),
('social', 48, 1, 'O‘yinda boshqa kimdir bo‘lib o‘ynaydi (o‘qituvchi, qahramon, kuchuk)', 'В игре изображает кого-то другого (учителя, супергероя, собаку)', 'Pretends to be something else during play, like a teacher, superhero or dog', 'published'),
('social', 48, 2, 'Atrofda bolalar bo‘lmasa, ular bilan o‘ynashni so‘raydi', 'Просится поиграть с детьми, если рядом никого нет', 'Asks to go play with children if none are around', 'published'),
('social', 48, 3, 'Xafa bo‘lgan yoki biror joyi og‘rigan odamni yupatadi', 'Утешает тех, кому больно или грустно', 'Comforts others who are hurt or sad, like hugging a crying friend', 'published'),
('thinking', 48, 1, 'Narsalarning bir nechta rangini aytadi', 'Называет несколько цветов предметов', 'Names a few colours of items', 'published'),
('thinking', 48, 2, 'Yaxshi biladigan ertakda keyin nima bo‘lishini aytadi', 'Говорит, что будет дальше в знакомой сказке', 'Tells what comes next in a well-known story', 'published'),
('thinking', 48, 3, 'Kamida uch qismli odam rasmini chizadi', 'Рисует человека хотя бы с тремя частями тела', 'Draws a person with three or more body parts', 'published'),
('movement', 48, 1, 'Katta to‘pni ko‘pincha ilib oladi', 'Чаще всего ловит большой мяч', 'Catches a large ball most of the time', 'published'),
('movement', 48, 2, 'Kattalar qarab turganda o‘ziga ovqat soladi yoki suv quyadi', 'Сам накладывает еду или наливает воду под присмотром взрослого', 'Serves themselves food or pours water, with adult supervision', 'published'),
('movement', 48, 3, 'Qalamni musht bilan emas, barmoqlari bilan ushlaydi', 'Держит карандаш пальцами, а не кулаком', 'Holds a crayon or pencil between fingers and thumb, not in a fist', 'published'),

('communication', 60, 1, 'Eshitgan yoki o‘ylab topgan, kamida ikki voqeali hikoyani aytib beradi', 'Пересказывает услышанную или придуманную историю хотя бы с двумя событиями', 'Tells a story they heard or made up with at least two events', 'published'),
('communication', 60, 2, 'O‘qib bergan kitobingiz haqidagi oddiy savollarga javob beradi', 'Отвечает на простые вопросы о прочитанной книге', 'Answers simple questions about a book or story after you read or tell it', 'published'),
('communication', 60, 3, 'Suhbatni uch martadan ko‘p navbatma-navbat davom ettiradi', 'Поддерживает разговор больше чем из трёх реплик', 'Keeps a conversation going with more than three back-and-forth exchanges', 'published'),
('social', 60, 1, 'Boshqa bolalar bilan o‘ynaganda qoidaga amal qiladi yoki navbat kutadi', 'Соблюдает правила и ждёт своей очереди в игре с детьми', 'Follows rules or takes turns when playing games with other children', 'published'),
('social', 60, 2, 'Siz uchun qo‘shiq aytadi, raqsga tushadi yoki sahna ko‘rsatadi', 'Поёт, танцует или разыгрывает сценки для вас', 'Sings, dances or acts for you', 'published'),
('social', 60, 3, 'Uyda oddiy yumushlarni bajaradi (dasturxon tuzashga yordamlashadi)', 'Выполняет простые домашние дела (помогает накрыть на стол)', 'Does simple chores at home, like helping to set the table', 'published'),
('thinking', 60, 1, '10 gacha sanaydi', 'Считает до 10', 'Counts to 10', 'published'),
('thinking', 60, 2, '«Kecha», «ertaga», «ertalab» kabi vaqt so‘zlarini ishlatadi', 'Использует слова о времени: «вчера», «завтра», «утром»', 'Uses words about time, like “yesterday”, “tomorrow” or “morning”', 'published'),
('thinking', 60, 3, 'Mashg‘ulot paytida 5–10 daqiqa diqqatini jamlaydi', 'Удерживает внимание 5–10 минут во время занятия', 'Pays attention for 5 to 10 minutes during activities', 'published'),
('movement', 60, 1, 'Bir oyoqda sakraydi', 'Прыгает на одной ноге', 'Hops on one foot', 'published'),
('movement', 60, 2, 'Ba’zi tugmalarni o‘zi qadaydi', 'Сам застёгивает некоторые пуговицы', 'Buttons some buttons', 'published');
