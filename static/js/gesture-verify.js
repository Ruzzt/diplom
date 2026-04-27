// Верификация жестом для опасных действий

const VERIFY_CONFIRM_COUNT = 4;

const GESTURE_EMOJIS = {
    'thumbs_up':  '👍',
    'peace':      '✌️',
    'open_palm':  '✋',
    'one_finger': '☝️'
};

// Определяет какой жест показан
function verifyDetectGesture(lm) {
    const thumbTipDist = Math.hypot(lm[4].x - lm[0].x, lm[4].y - lm[0].y);
    const thumbIpDist  = Math.hypot(lm[3].x - lm[0].x, lm[3].y - lm[0].y);
    const thumb = thumbTipDist > thumbIpDist * 1.1;
    const index  = lm[8].y  < lm[6].y;
    const middle = lm[12].y < lm[10].y;
    const ring   = lm[16].y < lm[14].y;
    const pinky  = lm[20].y < lm[18].y;

    if (thumb && !index && !middle && !ring && !pinky) return 'thumbs_up';
    if (!thumb && index && middle && !ring && !pinky) return 'peace';
    if (thumb && index && middle && ring && pinky) return 'open_palm';
    if (!thumb && index && !middle && !ring && !pinky) return 'one_finger';
    return null;
}

// Главная функция — показывает модалку, возвращает action_token
function requestGestureVerification() {
    return new Promise(async (resolve, reject) => {
        const modal = document.getElementById('gestureVerifyModal');
        const bsModal = new bootstrap.Modal(modal);
        const video = document.getElementById('gestureVerifyVideo');
        const emojiEl = document.getElementById('gestureVerifyEmoji');
        const nameEl = document.getElementById('gestureVerifyName');
        const statusEl = document.getElementById('gestureVerifyStatus');
        const progressBar = document.getElementById('gestureVerifyProgressBar');

        let cancelled = false;
        let stream = null;
        let intervalId = null;

        function cleanup() {
            cancelled = true;
            if (intervalId) clearInterval(intervalId);
            if (stream) stream.getTracks().forEach(t => t.stop());
            video.srcObject = null;
        }

        modal.addEventListener('hidden.bs.modal', function onHide() {
            modal.removeEventListener('hidden.bs.modal', onHide);
            cleanup();
            reject('cancelled');
        }, { once: true });

        try {
            // Запрашиваем жест
            statusEl.textContent = 'Загрузка...';
            emojiEl.textContent = '⏳';
            nameEl.textContent = '';
            progressBar.style.width = '0%';

            const challengeRes = await fetch('/api/gesture-verify-challenge');
            const challenge = await challengeRes.json();

            emojiEl.textContent = GESTURE_EMOJIS[challenge.gesture] || '🤚';
            nameEl.textContent = challenge.label;

            // Загружаем модель
            statusEl.textContent = 'Загрузка модели жестов...';
            let handLandmarker;
            if (window._loadHandLandmarker) {
                handLandmarker = await window._loadHandLandmarker();
            } else if (window.handLandmarker) {
                handLandmarker = window.handLandmarker;
            } else {
                statusEl.textContent = 'Модель жестов недоступна';
                return;
            }

            // Камера
            statusEl.textContent = 'Запуск камеры...';
            stream = await navigator.mediaDevices.getUserMedia({
                video: { width: 320, height: 240, facingMode: 'user' }
            });
            video.srcObject = stream;

            bsModal.show();

            statusEl.textContent = 'Покажите жест камере';

            let matchCount = 0;

            intervalId = setInterval(async () => {
                if (cancelled || video.readyState < 2) return;

                try {
                    const results = handLandmarker.detectForVideo(video, performance.now());

                    if (results.landmarks && results.landmarks.length > 0) {
                        const detected = verifyDetectGesture(results.landmarks[0]);

                        if (detected === challenge.gesture) {
                            matchCount++;
                            const pct = Math.min(100, (matchCount / VERIFY_CONFIRM_COUNT) * 100);
                            progressBar.style.width = pct + '%';
                            statusEl.textContent = `Распознаю... (${matchCount}/${VERIFY_CONFIRM_COUNT})`;

                            if (matchCount >= VERIFY_CONFIRM_COUNT) {
                                clearInterval(intervalId);
                                intervalId = null;
                                statusEl.textContent = 'Жест подтверждён! Отправка...';

                                // Получаем action_token
                                const confirmRes = await fetch('/api/gesture-verify-confirm', {
                                    method: 'POST',
                                    headers: { 'Content-Type': 'application/json' },
                                    body: JSON.stringify({ gesture_token: challenge.token })
                                });
                                const confirmData = await confirmRes.json();

                                cleanup();
                                bsModal.hide();

                                if (confirmData.action_token) {
                                    resolve(confirmData.action_token);
                                } else {
                                    reject('no token');
                                }
                            }
                        } else {
                            matchCount = Math.max(0, matchCount - 1);
                            const pct = Math.min(100, (matchCount / VERIFY_CONFIRM_COUNT) * 100);
                            progressBar.style.width = pct + '%';
                            if (detected) {
                                statusEl.textContent = `Вы показываете ${GESTURE_EMOJIS[detected] || '?'} — нужен другой!`;
                            }
                        }
                    } else {
                        matchCount = Math.max(0, matchCount - 1);
                        progressBar.style.width = Math.min(100, (matchCount / VERIFY_CONFIRM_COUNT) * 100) + '%';
                        statusEl.textContent = 'Поднесите руку к камере';
                    }
                } catch (e) {
                    // пропускаем ошибки кадров
                }
            }, 500);

        } catch (err) {
            cleanup();
            bsModal.hide();
            reject(err);
        }
    });
}

// Опасное действие с подтверждением жестом
async function dangerousAction(url, method) {
    try {
        const actionToken = await requestGestureVerification();
        const form = document.createElement('form');
        form.method = method;
        form.action = url;
        const input = document.createElement('input');
        input.type = 'hidden';
        input.name = 'action_token';
        input.value = actionToken;
        form.appendChild(input);
        document.body.appendChild(form);
        form.submit();
    } catch (e) {
        // Пользователь отменил
    }
}

// Опасная отправка формы с подтверждением жестом
async function dangerousFormAction(formEl) {
    try {
        const actionToken = await requestGestureVerification();
        const input = document.createElement('input');
        input.type = 'hidden';
        input.name = 'action_token';
        input.value = actionToken;
        formEl.appendChild(input);
        formEl.submit();
    } catch (e) {
        // Пользователь отменил
    }
}
