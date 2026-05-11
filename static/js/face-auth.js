let video;
let faceDescriptor = null;
let handsModel = null;
let gestureConfirmed = false;
let gestureMatchCount = 0;
let handsReady = false;
const GESTURE_CONFIRM_COUNT = 4;

const gestureEmojis = {
    'thumbs_up':  '👍',
    'peace':      '✌️',
    'open_palm':  '✋',
    'one_finger': '☝️'
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

        // Для входа — ждём загрузку модели жестов (грузится в HTML module script)
        if (mode === 'login') {
            statusText.textContent = 'Ожидание модели жестов...';
            await waitForHandModel();
        }

        statusText.textContent = 'Запуск камеры...';

        const stream = await navigator.mediaDevices.getUserMedia({
            video: { width: 640, height: 480, facingMode: 'user' }
        });
        video.srcObject = stream;

        video.addEventListener('play', () => {
            statusEl.className = 'alert alert-success';

            if (mode === 'login') {
                statusText.textContent = 'Камера активна. Покажите жест и смотрите в камеру.';
                if (handsReady) {
                    startGestureDetection();
                }
            } else {
                statusText.textContent = 'Камера активна. Посмотрите в камеру.';
                document.getElementById('registerBtn').disabled = false;
            }

            detectFaceLoop();
        });

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

function waitForHandModel() {
    return new Promise((resolve) => {
        // Модель уже загружена?
        if (window.handLandmarker) {
            handsModel = window.handLandmarker;
            handsReady = true;
            console.log('Модель жестов уже готова');
            resolve();
            return;
        }

        // Ждём событие загрузки
        window.addEventListener('hand-model-ready', () => {
            handsModel = window.handLandmarker;
            handsReady = true;
            console.log('Модель жестов получена');
            resolve();
        });

        // Если ошибка — продолжаем без жестов
        window.addEventListener('hand-model-error', () => {
            console.error('Модель жестов не загрузилась');
            resolve();
        });

        // Таймаут 30 секунд
        setTimeout(() => {
            if (!handsReady) {
                console.warn('Таймаут загрузки модели жестов');
                const gestureStatusEl = document.getElementById('gestureStatus');
                if (gestureStatusEl) {
                    gestureStatusEl.textContent = '⚠️ Модель жестов не загрузилась. Обновите страницу (Ctrl+Shift+R).';
                    gestureStatusEl.className = 'gesture-status warning';
                }
            }
            resolve();
        }, 30000);
    });
}

function startGestureDetection() {
    console.log('Запуск распознавания жестов');
    const gestureStatusEl = document.getElementById('gestureStatus');
    gestureStatusEl.textContent = 'Покажите жест камере';

    setInterval(() => {
        if (!handsModel || !handsReady || video.readyState < 2 || gestureConfirmed) return;

        try {
            const results = handsModel.detectForVideo(video, performance.now());
            processHandResults(results);
        } catch (e) {
            console.error('Ошибка детекции:', e);
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

function processHandResults(results) {
    if (gestureConfirmed) return;

    const gestureStatusEl = document.getElementById('gestureStatus');
    const requiredGesture = (typeof GESTURE_DATA !== 'undefined') ? GESTURE_DATA.gesture : null;
    if (!requiredGesture) return;

    if (results.landmarks && results.landmarks.length > 0) {
        const landmarks = results.landmarks[0];
        const detected = detectGesture(landmarks);

        if (detected === requiredGesture) {
            gestureMatchCount++;
            gestureStatusEl.textContent = `Распознаю жест... (${gestureMatchCount}/${GESTURE_CONFIRM_COUNT})`;
            gestureStatusEl.className = 'gesture-status detecting';
            updateGestureProgress();

            if (gestureMatchCount >= GESTURE_CONFIRM_COUNT) {
                gestureConfirmed = true;
                gestureStatusEl.textContent = 'Жест подтверждён! Нажмите кнопку входа.';
                gestureStatusEl.className = 'gesture-status success';

                document.getElementById('gestureChallenge').classList.add('confirmed');
                updateGestureProgress();
                document.getElementById('loginBtn').disabled = false;
            }
        } else {
            gestureMatchCount = Math.max(0, gestureMatchCount - 1);
            updateGestureProgress();
            if (detected) {
                const shownEmoji = gestureEmojis[detected] || '?';
                gestureStatusEl.textContent = `Вы показываете ${shownEmoji} — нужен другой жест!`;
                gestureStatusEl.className = 'gesture-status warning';
            } else {
                gestureStatusEl.textContent = 'Покажите жест чётче';
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

function detectGesture(lm) {
    const fingers = getExtendedFingers(lm);
    const [thumb, index, middle, ring, pinky] = fingers;

    if (thumb && !index && !middle && !ring && !pinky) return 'thumbs_up';
    if (!thumb && index && middle && !ring && !pinky) return 'peace';
    if (thumb && index && middle && ring && pinky) return 'open_palm';
    if (!thumb && index && !middle && !ring && !pinky) return 'one_finger';

    return null;
}

function getExtendedFingers(lm) {
    const thumbTipDist = Math.hypot(lm[4].x - lm[0].x, lm[4].y - lm[0].y);
    const thumbIpDist  = Math.hypot(lm[3].x - lm[0].x, lm[3].y - lm[0].y);
    const thumb = thumbTipDist > thumbIpDist * 1.1;

    const index  = lm[8].y  < lm[6].y;
    const middle = lm[12].y < lm[10].y;
    const ring   = lm[16].y < lm[14].y;
    const pinky  = lm[20].y < lm[18].y;

    return [thumb, index, middle, ring, pinky];
}

// ==================== Распознавание лица ====================

async function detectFaceLoop() {
    const overlay = document.getElementById('overlay');

    // Ждём пока видео получит реальные размеры
    if (video.videoWidth === 0) {
        setTimeout(detectFaceLoop, 100);
        return;
    }

    setInterval(async () => {
        // Каждый раз берём актуальный отображаемый размер видео
        const rect = video.getBoundingClientRect();
        const displaySize = { width: rect.width, height: rect.height };

        // Синхронизируем canvas с отображаемым размером
        if (overlay.width !== displaySize.width || overlay.height !== displaySize.height) {
            overlay.width = displaySize.width;
            overlay.height = displaySize.height;
            // Убираем CSS-размеры, чтобы canvas совпадал пиксель-в-пиксель
            overlay.style.width = displaySize.width + 'px';
            overlay.style.height = displaySize.height + 'px';
        }

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

async function captureFaceData() {
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

    // Извлекаем landmarks (68 точек) — нормализованные координаты
    const positions = detection.landmarks.positions;
    const box = detection.detection.box;
    const landmarks = positions.map(pt => ({
        x: (pt.x - box.x) / box.width,
        y: (pt.y - box.y) / box.height
    }));

    return {
        descriptor: Array.from(detection.descriptor),
        landmarks: landmarks
    };
}

// ==================== Вход ====================

async function handleLogin() {
    const btn = document.getElementById('loginBtn');
    const statusEl = document.getElementById('status');
    const statusText = document.getElementById('statusText');

    if (!gestureConfirmed) {
        statusEl.className = 'alert alert-danger';
        statusText.textContent = 'Сначала покажите требуемый жест камере';
        return;
    }

    btn.disabled = true;
    btn.innerHTML = '<span class="spinner-border spinner-border-sm"></span> Распознавание...';

    const faceData = await captureFaceData();
    if (!faceData) {
        btn.disabled = false;
        btn.innerHTML = '<i class="bi bi-person-check"></i> Войти по лицу';
        return;
    }

    const gestureToken = (typeof GESTURE_DATA !== 'undefined') ? GESTURE_DATA.token : '';

    try {
        const response = await fetch('/api/login', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                face_descriptor: faceData.descriptor,
                face_landmarks: faceData.landmarks,
                gesture_token: gestureToken
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

    const faceData = await captureFaceData();
    if (!faceData) {
        btn.disabled = false;
        btn.innerHTML = '<i class="bi bi-person-check"></i> Зарегистрироваться';
        return;
    }

    try {
        const response = await fetch('/api/register', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ name, email, role, face_descriptor: faceData.descriptor, face_landmarks: faceData.landmarks })
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
