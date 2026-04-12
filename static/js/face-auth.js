let video;
let faceDescriptor = null;
let handsModel = null;
let gestureChallenge = null;   // {gesture, label, token}
let gestureConfirmed = false;
let gestureMatchCount = 0;
let handsReady = false;
const GESTURE_CONFIRM_COUNT = 4; // 4 совпадения подряд (~2 сек)

const gestureEmojis = {
    'thumbs_up':  '👍',
    'peace':      '✌️',
    'open_palm':  '✋',
    'one_finger': '☝️'
};

const gestureNames = {
    'thumbs_up':  'Большой палец вверх',
    'peace':      'Знак мира (два пальца)',
    'open_palm':  'Открытая ладонь',
    'one_finger': 'Один палец вверх'
};

async function initFaceAuth(mode) {
    video = document.getElementById('video');
    const statusEl = document.getElementById('status');
    const statusText = document.getElementById('statusText');

    try {
        const MODEL_URL = 'https://cdn.jsdelivr.net/npm/@vladmandic/face-api@1.7.14/model';

        statusText.textContent = 'Загрузка моделей распознавания лиц...';

        await Promise.all([
            faceapi.nets.tinyFaceDetector.loadFromUri(MODEL_URL),
            faceapi.nets.faceLandmark68Net.loadFromUri(MODEL_URL),
            faceapi.nets.faceRecognitionNet.loadFromUri(MODEL_URL),
        ]);

        statusText.textContent = 'Модели загружены. Запуск камеры...';

        // Для входа — загружаем модель жестов и запрашиваем задание параллельно с камерой
        if (mode === 'login') {
            initHandsModel();
            fetchGestureChallenge();
        }

        // Запускаем видеопоток с камеры
        const stream = await navigator.mediaDevices.getUserMedia({
            video: { width: 640, height: 480, facingMode: 'user' }
        });
        video.srcObject = stream;

        video.addEventListener('play', () => {
            statusEl.className = 'alert alert-success';
            statusText.textContent = 'Камера активна. Посмотрите в камеру.';

            if (mode === 'login') {
                startGestureDetection();
            } else {
                document.getElementById('registerBtn').disabled = false;
            }

            detectFaceLoop();
        });

        // Привязываем кнопки
        if (mode === 'login') {
            document.getElementById('loginBtn').addEventListener('click', handleLogin);
        } else {
            document.getElementById('registerBtn').addEventListener('click', handleRegister);
        }

    } catch (error) {
        console.error('Ошибка инициализации:', error);
        statusEl.className = 'alert alert-danger';
        statusText.textContent = 'Ошибка: ' + error.message;
    }
}

// ==================== Распознавание жестов ====================

function initHandsModel() {
    const gestureStatusEl = document.getElementById('gestureStatus');

    if (typeof Hands === 'undefined') {
        console.error('MediaPipe Hands не загружен');
        gestureStatusEl.textContent = 'Ошибка загрузки модели жестов. Обновите страницу.';
        gestureStatusEl.className = 'gesture-status warning';
        return;
    }

    try {
        handsModel = new Hands({
            locateFile: (file) => `https://cdn.jsdelivr.net/npm/@mediapipe/hands@0.4.1675469240/${file}`
        });

        handsModel.setOptions({
            maxNumHands: 1,
            modelComplexity: 1,
            minDetectionConfidence: 0.7,
            minTrackingConfidence: 0.5
        });

        handsModel.onResults(onHandResults);
        handsReady = true;

        gestureStatusEl.textContent = 'Модель жестов загружена. Покажите жест камере.';
    } catch (err) {
        console.error('Ошибка инициализации MediaPipe Hands:', err);
        gestureStatusEl.textContent = 'Ошибка модели жестов: ' + err.message;
        gestureStatusEl.className = 'gesture-status warning';
    }
}

async function fetchGestureChallenge() {
    const gestureStatusEl = document.getElementById('gestureStatus');
    const gestureEmoji = document.getElementById('gestureEmoji');
    const gestureIcon = document.getElementById('gestureIcon');

    try {
        const res = await fetch('/api/gesture-challenge');
        if (!res.ok) throw new Error('Сервер вернул ошибку');

        gestureChallenge = await res.json();

        gestureEmoji.textContent = gestureEmojis[gestureChallenge.gesture] || '🤚';
        gestureIcon.textContent = gestureNames[gestureChallenge.gesture] || gestureChallenge.label;

        if (handsReady) {
            gestureStatusEl.textContent = 'Покажите этот жест камере';
        }
    } catch (err) {
        console.error('Ошибка загрузки жеста:', err);
        gestureEmoji.textContent = '❌';
        gestureIcon.textContent = 'Ошибка загрузки';
        gestureStatusEl.textContent = 'Не удалось получить жест. Обновите страницу.';
        gestureStatusEl.className = 'gesture-status warning';
    }
}

function startGestureDetection() {
    setInterval(async () => {
        if (handsModel && handsReady && video.readyState >= 2 && !gestureConfirmed) {
            try {
                await handsModel.send({ image: video });
            } catch (e) {
                // пропускаем ошибки кадров
            }
        }
    }, 500);
}

function updateGestureProgress() {
    const bar = document.getElementById('gestureProgressBar');
    if (bar) {
        const percent = Math.min(100, (gestureMatchCount / GESTURE_CONFIRM_COUNT) * 100);
        bar.style.width = percent + '%';
    }
}

function onHandResults(results) {
    if (gestureConfirmed) return;

    const gestureStatusEl = document.getElementById('gestureStatus');

    if (results.multiHandLandmarks && results.multiHandLandmarks.length > 0) {
        const landmarks = results.multiHandLandmarks[0];
        const detected = detectGesture(landmarks);

        if (detected === gestureChallenge?.gesture) {
            gestureMatchCount++;
            gestureStatusEl.textContent = `Распознаю жест... (${gestureMatchCount}/${GESTURE_CONFIRM_COUNT})`;
            gestureStatusEl.className = 'gesture-status detecting';
            updateGestureProgress();

            if (gestureMatchCount >= GESTURE_CONFIRM_COUNT) {
                gestureConfirmed = true;
                gestureStatusEl.textContent = '✅ Жест подтверждён! Нажмите кнопку входа.';
                gestureStatusEl.className = 'gesture-status success';

                const challengeEl = document.getElementById('gestureChallenge');
                challengeEl.classList.add('confirmed');

                document.getElementById('gestureEmoji').textContent = '✅';
                updateGestureProgress();

                document.getElementById('loginBtn').disabled = false;
            }
        } else {
            gestureMatchCount = Math.max(0, gestureMatchCount - 1);
            updateGestureProgress();
            if (detected) {
                const shownEmoji = gestureEmojis[detected] || '';
                gestureStatusEl.textContent = `Вы показываете ${shownEmoji} — нужен другой жест!`;
                gestureStatusEl.className = 'gesture-status warning';
            } else {
                gestureStatusEl.textContent = 'Покажите жест камере';
                gestureStatusEl.className = 'gesture-status';
            }
        }
    } else {
        gestureMatchCount = Math.max(0, gestureMatchCount - 1);
        updateGestureProgress();
        gestureStatusEl.textContent = 'Поднесите руку к камере';
        gestureStatusEl.className = 'gesture-status';
    }
}

// Определяет какой жест показан по ориентирам руки
function detectGesture(lm) {
    const fingers = getExtendedFingers(lm);
    const [thumb, index, middle, ring, pinky] = fingers;

    // 👍 Большой палец вверх: только большой палец
    if (thumb && !index && !middle && !ring && !pinky) return 'thumbs_up';

    // ✌️ Знак мира: указательный + средний
    if (!thumb && index && middle && !ring && !pinky) return 'peace';

    // ✋ Открытая ладонь: все пальцы
    if (thumb && index && middle && ring && pinky) return 'open_palm';

    // ☝️ Один палец: только указательный
    if (!thumb && index && !middle && !ring && !pinky) return 'one_finger';

    return null;
}

// Определяет какие пальцы разогнуты (вытянуты)
function getExtendedFingers(lm) {
    // Большой палец: кончик (4) дальше от запястья (0) чем сустав (3)
    const thumbTipDist = Math.hypot(lm[4].x - lm[0].x, lm[4].y - lm[0].y);
    const thumbIpDist  = Math.hypot(lm[3].x - lm[0].x, lm[3].y - lm[0].y);
    const thumb = thumbTipDist > thumbIpDist * 1.1;

    // Остальные пальцы: кончик выше (меньше y) чем средний сустав (PIP)
    const index  = lm[8].y  < lm[6].y;
    const middle = lm[12].y < lm[10].y;
    const ring   = lm[16].y < lm[14].y;
    const pinky  = lm[20].y < lm[18].y;

    return [thumb, index, middle, ring, pinky];
}

// ==================== Распознавание лица ====================

async function detectFaceLoop() {
    const overlay = document.getElementById('overlay');
    const displaySize = { width: video.videoWidth, height: video.videoHeight };

    if (displaySize.width === 0) {
        setTimeout(detectFaceLoop, 100);
        return;
    }

    overlay.width = displaySize.width;
    overlay.height = displaySize.height;
    faceapi.matchDimensions(overlay, displaySize);

    setInterval(async () => {
        const detections = await faceapi.detectAllFaces(
            video,
            new faceapi.TinyFaceDetectorOptions()
        ).withFaceLandmarks();

        const resized = faceapi.resizeResults(detections, displaySize);
        const ctx = overlay.getContext('2d');
        ctx.clearRect(0, 0, overlay.width, overlay.height);
        faceapi.draw.drawDetections(overlay, resized);
    }, 300);
}

async function captureFaceDescriptor() {
    const statusEl = document.getElementById('status');
    const statusText = document.getElementById('statusText');

    statusEl.className = 'alert alert-info';
    statusText.textContent = 'Распознавание лица...';

    const detection = await faceapi.detectSingleFace(
        video,
        new faceapi.TinyFaceDetectorOptions()
    ).withFaceLandmarks().withFaceDescriptor();

    if (!detection) {
        statusEl.className = 'alert alert-danger';
        statusText.textContent = 'Лицо не обнаружено. Убедитесь, что лицо видно камере.';
        return null;
    }

    statusEl.className = 'alert alert-success';
    statusText.textContent = 'Лицо распознано!';

    return Array.from(detection.descriptor);
}

// ==================== Вход ====================

async function handleLogin() {
    const btn = document.getElementById('loginBtn');
    const statusEl = document.getElementById('status');
    const statusText = document.getElementById('statusText');

    // Проверяем, что жест подтверждён
    if (!gestureConfirmed) {
        statusEl.className = 'alert alert-danger';
        statusText.textContent = 'Сначала покажите требуемый жест камере';
        return;
    }

    btn.disabled = true;
    btn.innerHTML = '<span class="spinner-border spinner-border-sm"></span> Распознавание...';

    const descriptor = await captureFaceDescriptor();
    if (!descriptor) {
        btn.disabled = false;
        btn.innerHTML = '<i class="bi bi-person-check"></i> Войти по лицу';
        return;
    }

    try {
        const response = await fetch('/api/login', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                face_descriptor: descriptor,
                gesture_token: gestureChallenge?.token || ''
            })
        });

        const data = await response.json();

        if (response.ok) {
            statusEl.className = 'alert alert-success';
            const roleLabels = { admin: 'Администратор', manager: 'Менеджер', viewer: 'Зритель' };
            statusText.textContent = `Добро пожаловать, ${data.user}!`;

            const userInfo = document.getElementById('userInfo');
            if (userInfo) {
                userInfo.classList.remove('d-none');
                document.getElementById('userName').textContent = data.user;
                document.getElementById('userRole').textContent = roleLabels[data.role] || data.role;
            }

            setTimeout(() => window.location.href = '/dashboard', 2000);
        } else {
            statusEl.className = 'alert alert-danger';
            statusText.textContent = data.error;
            btn.disabled = false;
            btn.innerHTML = '<i class="bi bi-person-check"></i> Войти по лицу';
        }
    } catch (error) {
        statusEl.className = 'alert alert-danger';
        statusText.textContent = 'Ошибка сети: ' + error.message;
        btn.disabled = false;
        btn.innerHTML = '<i class="bi bi-person-check"></i> Войти по лицу';
    }
}

// ==================== Регистрация ====================

async function handleRegister() {
    const btn = document.getElementById('registerBtn');
    const statusEl = document.getElementById('status');
    const statusText = document.getElementById('statusText');

    const name = document.getElementById('name').value.trim();
    const email = document.getElementById('email').value.trim();
    const role = document.getElementById('role').value;

    if (!name || !email) {
        statusEl.className = 'alert alert-danger';
        statusText.textContent = 'Заполните имя и email';
        return;
    }

    btn.disabled = true;
    btn.innerHTML = '<span class="spinner-border spinner-border-sm"></span> Регистрация...';

    const descriptor = await captureFaceDescriptor();
    if (!descriptor) {
        btn.disabled = false;
        btn.innerHTML = '<i class="bi bi-person-check"></i> Зарегистрироваться';
        return;
    }

    try {
        const response = await fetch('/api/register', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ name, email, role, face_descriptor: descriptor })
        });

        const data = await response.json();

        if (response.ok) {
            if (data.approved) {
                statusEl.className = 'alert alert-success';
                statusText.textContent = `Регистрация успешна! Добро пожаловать, ${data.user}!`;
                setTimeout(() => window.location.href = '/dashboard', 1000);
            } else {
                statusEl.className = 'alert alert-warning';
                statusText.textContent = data.message;
                btn.innerHTML = '<i class="bi bi-hourglass-split"></i> Заявка отправлена';
            }
        } else {
            statusEl.className = 'alert alert-danger';
            statusText.textContent = data.error;
            btn.disabled = false;
            btn.innerHTML = '<i class="bi bi-person-check"></i> Зарегистрироваться';
        }
    } catch (error) {
        statusEl.className = 'alert alert-danger';
        statusText.textContent = 'Ошибка сети: ' + error.message;
        btn.disabled = false;
        btn.innerHTML = '<i class="bi bi-person-check"></i> Зарегистрироваться';
    }
}
