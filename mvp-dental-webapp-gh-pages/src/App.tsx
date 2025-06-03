import React from 'react';
import { Container, Box, Typography, Button, CssBaseline, ThemeProvider, createTheme } from '@mui/material';
import MedicalServicesIcon from '@mui/icons-material/MedicalServices';
import { LocalizationProvider, DatePicker, PickersDay } from '@mui/x-date-pickers';
import { AdapterDateFns } from '@mui/x-date-pickers/AdapterDateFns';
import { ru } from 'date-fns/locale';
import { isSameDay, addDays, startOfToday } from 'date-fns';
import TextField from '@mui/material/TextField';

const theme = createTheme({
  palette: {
    mode: 'light',
    primary: {
      main: '#1976d2',
    },
    secondary: {
      main: '#43a047',
    },
    background: {
      default: '#f7fafd',
    },
  },
  typography: {
    fontFamily: 'Inter, Roboto, Arial, sans-serif',
    h4: {
      fontWeight: 700,
    },
    h6: {
      fontWeight: 400,
    },
  },
});

type Step = 'main' | 'service' | 'date' | 'time' | 'confirm' | 'success';

function App() {
  // Состояния
  const [tgUser, setTgUser] = React.useState<{id: number; first_name: string; last_name?: string} | null>(null);
  const [step, setStep] = React.useState<Step>('main');
  const [services, setServices] = React.useState<any[]>([]);
  const [loading, setLoading] = React.useState(false);
  const [error, setError] = React.useState<string | null>(null);
  const [selectedService, setSelectedService] = React.useState<any | null>(null);
  const [selectedDate, setSelectedDate] = React.useState<Date | null>(null);
  const [selectedTime, setSelectedTime] = React.useState<string | null>(null);
  const [phone, setPhone] = React.useState('');
  const [phoneError, setPhoneError] = React.useState('');
  const [smsSent, setSmsSent] = React.useState(false);
  const [smsCode, setSmsCode] = React.useState('');
  const [smsError, setSmsError] = React.useState('');

  // Получение пользователя из Telegram WebApp API
  React.useEffect(() => {
    // @ts-ignore
    if (window.Telegram && window.Telegram.WebApp) {
      // @ts-ignore
      const webapp = window.Telegram.WebApp;
      webapp.ready();
      webapp.expand();
      
      // @ts-ignore
      const user = webapp.initDataUnsafe?.user;
      if (user) {
        setTgUser(user);
      }
    }
  }, []);

  // Загрузка услуг
  React.useEffect(() => {
    if (step === 'service') {
      setLoading(true);
      setError(null);
      fetch('http://localhost:8080/api/services')
        .then((res) => {
          if (!res.ok) throw new Error('Ошибка загрузки услуг');
          return res.json();
        })
        .then((data) => {
          console.log('Loaded services:', data);
          setServices(data);
        })
        .catch((e) => {
          console.error('Error loading services:', e);
          setError(e.message);
        })
        .finally(() => setLoading(false));
    }
  }, [step]);

  // Календарь (заглушка)
  const today = startOfToday();
  const availableDays = React.useMemo(() => {
    // Массив: {date: Date, status: 'free' | 'busy' | 'disabled'}
    return Array.from({ length: 14 }).map((_, i) => {
      const date = addDays(today, i);
      // Случайная генерация доступности (для демо)
      const rnd = Math.random();
      let status: 'free' | 'busy' | 'disabled' = 'free';
      if (rnd < 0.2) status = 'busy';
      if (rnd < 0.05) status = 'disabled';
      return { date, status };
    });
  }, [today]);

  // Генерация слотов времени (заглушка)
  const timeSlots = React.useMemo(() => {
    // 9:00 - 18:00, шаг 30 мин
    const slots: { time: string; status: 'free' | 'busy' }[] = [];
    for (let h = 9; h < 18; h++) {
      for (let m = 0; m < 60; m += 30) {
        const time = `${h.toString().padStart(2, '0')}:${m.toString().padStart(2, '0')}`;
        // Случайная занятость
        const status = Math.random() < 0.2 ? 'busy' : 'free';
        slots.push({ time, status });
      }
    }
    return slots;
  }, [selectedDate, selectedService]);

  function renderDay(day: Date, value: Date | null, DayComponentProps: any) {
    const found = availableDays.find((d) => isSameDay(d.date, day));
    let color: string | undefined = undefined;
    if (found) {
      if (found.status === 'free') color = '#43a047';
      if (found.status === 'busy') color = '#e53935';
      if (found.status === 'disabled') color = '#bdbdbd';
    }
    return (
      <PickersDay
        {...DayComponentProps}
        sx={color ? { bgcolor: color, color: '#fff', '&:hover': { bgcolor: color } } : {}}
        disabled={found?.status === 'disabled'}
      />
    );
  }

  // Валидация телефона
  function normalizePhone(phone: string) {
    // Оставляем только цифры
    return phone.replace(/\D/g, '');
  }

  function formatPhoneForServer(phone: string) {
    const digits = normalizePhone(phone);
    
    // Если длина 11 цифр
    if (digits.length === 11) {
      // Если начинается с 8, заменяем на +7
      if (digits.startsWith('8')) {
        return '+7' + digits.slice(1);
      }
      // Если начинается с 7, добавляем +
      if (digits.startsWith('7')) {
        return '+' + digits;
      }
    }
    return '';
  }

  function validatePhone(phone: string) {
    const digits = normalizePhone(phone);
    return digits.length === 11 && (digits.startsWith('7') || digits.startsWith('8'));
  }

  // --- UI шаги ---
  if (step === 'main') {
    return (
      <ThemeProvider theme={theme}>
        <CssBaseline />
        <Container maxWidth="sm" sx={{ minHeight: '100vh', display: 'flex', flexDirection: 'column', justifyContent: 'center' }}>
          <Box textAlign="center" mb={4}>
            <MedicalServicesIcon color="primary" sx={{ fontSize: 60, mb: 2 }} />
            <Typography variant="h4" gutterBottom>
              Добро пожаловать в стоматологию
            </Typography>
            {tgUser && (
              <Typography variant="h6" color="text.secondary" gutterBottom>
                {tgUser.first_name}{tgUser.last_name ? ` ${tgUser.last_name}` : ''}, рады видеть вас!
              </Typography>
            )}
            {!tgUser && (
              <Typography variant="h6" color="text.secondary" gutterBottom>
                Онлайн-запись на приём — быстро, удобно, современно
              </Typography>
            )}
          </Box>
          <Button
            variant="contained"
            color="primary"
            size="large"
            sx={{ borderRadius: 3, py: 1.5, fontWeight: 600, fontSize: 18 }}
            onClick={() => setStep('service')}
          >
            Записаться на приём
          </Button>
        </Container>
      </ThemeProvider>
    );
  }

  if (step === 'service') {
    return (
      <ThemeProvider theme={theme}>
        <CssBaseline />
        <Container maxWidth="sm" sx={{ minHeight: '100vh', display: 'flex', flexDirection: 'column', justifyContent: 'center' }}>
          <Typography variant="h5" gutterBottom>Выберите услугу</Typography>
          {loading && <Typography>Загрузка...</Typography>}
          {error && <Typography color="error">{error}</Typography>}
          {!loading && !error && services.length === 0 && (
            <Typography color="text.secondary">Нет доступных услуг</Typography>
          )}
          <Box display="flex" flexDirection="column" gap={2} mt={2}>
            {services.map((s) => (
              <Button
                key={s.ID}
                variant="outlined"
                color="primary"
                sx={{ justifyContent: 'flex-start', textAlign: 'left', borderRadius: 3, p: 2 }}
                onClick={() => {
                  setSelectedService(s);
                  setSelectedDate(null);
                  setSelectedTime(null);
                  setStep('date');
                }}
              >
                <Box>
                  <Typography fontWeight={600}>{s.Name}</Typography>
                  <Typography variant="body2" color="text.secondary">
                    {s.Duration} мин · {s.Price} ₽
                  </Typography>
                </Box>
              </Button>
            ))}
          </Box>
        </Container>
      </ThemeProvider>
    );
  }

  if (step === 'date' && selectedService) {
    return (
      <ThemeProvider theme={theme}>
        <CssBaseline />
        <LocalizationProvider dateAdapter={AdapterDateFns} adapterLocale={ru}>
          <Container maxWidth="sm" sx={{ minHeight: '100vh', display: 'flex', flexDirection: 'column', justifyContent: 'center' }}>
            <Typography variant="h5" gutterBottom>Выберите дату</Typography>
            <Typography variant="subtitle1" color="text.secondary" gutterBottom>
              {selectedService.name} · {selectedService.duration} мин · {selectedService.price} ₽
            </Typography>
            <Box mt={4}>
              <DatePicker
                label="Дата приёма"
                value={selectedDate}
                onChange={(date) => setSelectedDate(date)}
                disablePast
                slots={{ day: (props) => renderDay(props.day as Date, selectedDate, props) }}
                slotProps={{ textField: { fullWidth: true } }}
              />
            </Box>
            {selectedDate && (
              <Box mt={3}>
                <Button
                  variant="contained"
                  color="primary"
                  size="large"
                  sx={{ borderRadius: 3 }}
                  onClick={() => setStep('time')}
                >
                  Далее
                </Button>
              </Box>
            )}
          </Container>
        </LocalizationProvider>
      </ThemeProvider>
    );
  }

  if (step === 'time' && selectedService && selectedDate) {
    return (
      <ThemeProvider theme={theme}>
        <CssBaseline />
        <Container maxWidth="sm" sx={{ minHeight: '100vh', display: 'flex', flexDirection: 'column', justifyContent: 'center' }}>
          <Typography variant="h5" gutterBottom>Выберите время</Typography>
          <Typography variant="subtitle1" color="text.secondary" gutterBottom>
            {selectedService.name} · {selectedService.duration} мин · {selectedService.price} ₽<br/>
            {selectedDate.toLocaleDateString('ru-RU')}
          </Typography>
          <Box mt={3} display="flex" flexWrap="wrap" gap={2}>
            {timeSlots.map((slot) => (
              <Button
                key={slot.time}
                variant={slot.status === 'free' ? (selectedTime === slot.time ? 'contained' : 'outlined') : 'outlined'}
                color={slot.status === 'free' ? 'primary' : 'error'}
                disabled={slot.status === 'busy'}
                sx={{ minWidth: 100, mb: 1, borderRadius: 3, fontWeight: 600 }}
                onClick={() => {
                  setSelectedTime(slot.time);
                  setStep('confirm');
                }}
              >
                {slot.time}
              </Button>
            ))}
          </Box>
        </Container>
      </ThemeProvider>
    );
  }

  if (step === 'confirm' && selectedService && selectedDate && selectedTime) {
    return (
      <ThemeProvider theme={theme}>
        <CssBaseline />
        <Container maxWidth="sm" sx={{ minHeight: '100vh', display: 'flex', flexDirection: 'column', justifyContent: 'center' }}>
          <Typography variant="h5" gutterBottom>Подтверждение записи</Typography>
          <Typography variant="subtitle1" color="text.secondary" gutterBottom>
            {selectedService.name} · {selectedService.duration} мин · {selectedService.price} ₽<br/>
            {selectedDate.toLocaleDateString('ru-RU')} · {selectedTime}
          </Typography>
          {!smsSent ? (
            <Box mt={3}>
              <TextField
                label="Телефон"
                value={phone}
                onChange={e => {
                  const newValue = e.target.value;
                  // Оставляем только цифры
                  const digits = newValue.replace(/\D/g, '');
                  // Ограничиваем длину 11 цифрами
                  if (digits.length <= 11) {
                    setPhone(digits);
                  }
                }}
                fullWidth
                placeholder="Введите номер телефона"
                error={!!phoneError}
                helperText={phoneError || 'Введите 11 цифр номера телефона'}
                sx={{ mb: 2 }}
              />
              <Button
                variant="contained"
                color="primary"
                size="large"
                sx={{ borderRadius: 3 }}
                onClick={async () => {
                  const formattedPhone = formatPhoneForServer(phone);
                  console.log('Final formatted phone:', formattedPhone);
                  
                  if (!formattedPhone) {
                    setPhoneError('Введите корректный номер телефона');
                    return;
                  }
                  
                  setPhoneError('');
                  try {
                    const requestBody = { 
                      phone: formattedPhone,
                      telegram_id: tgUser?.id || 123456789 // Временный ID для тестирования
                    };
                    console.log('Sending request with body:', requestBody);
                    
                    const res = await fetch('http://localhost:8080/api/send_sms', {
                      method: 'POST',
                      headers: { 'Content-Type': 'application/json' },
                      body: JSON.stringify(requestBody)
                    });
                    
                    const data = await res.json();
                    console.log('Server response:', data);
                    
                    if (!data.ok) {
                      throw new Error(data.error || 'Ошибка отправки SMS');
                    }
                    
                    setSmsSent(true);
                    setSmsCode(data.code || '');
                  } catch (e: any) {
                    console.error('Error details:', e);
                    setPhoneError(e.message || 'Ошибка отправки SMS');
                  }
                }}
              >
                Получить код
              </Button>
            </Box>
          ) : (
            <Box mt={3}>
              <TextField
                label="Код из SMS"
                value={smsCode}
                onChange={e => setSmsCode(e.target.value)}
                fullWidth
                error={!!smsError}
                helperText={smsError || 'Введите код, отправленный на телефон'}
                sx={{ mb: 2 }}
              />
              <Button
                variant="contained"
                color="primary"
                size="large"
                sx={{ borderRadius: 3 }}
                disabled={smsCode.length !== 4}
                onClick={async () => {
                  setSmsError('');
                  try {
                    if (!selectedService?.ID) {
                      throw new Error('Не выбрана услуга');
                    }
                    
                    const requestBody = {
                      phone: formatPhoneForServer(phone),
                      telegram_id: tgUser?.id || 123456789,
                      name: tgUser?.first_name || 'Гость',
                      service_id: selectedService.ID,
                      date: selectedDate.toISOString().slice(0, 10),
                      time: selectedTime,
                      code: smsCode
                    };
                    
                    const res = await fetch('http://localhost:8080/api/bookings', {
                      method: 'POST',
                      headers: { 'Content-Type': 'application/json' },
                      body: JSON.stringify(requestBody)
                    });
                    
                    const data = await res.json();
                    if (!data.ok) throw new Error(data.error || 'Ошибка подтверждения');
                    setStep('success');
                  } catch (e: any) {
                    setSmsError(e.message || 'Ошибка подтверждения');
                  }
                }}
              >
                Подтвердить запись
              </Button>
            </Box>
          )}
        </Container>
      </ThemeProvider>
    );
  }

  if (step === 'success') {
    return (
      <ThemeProvider theme={theme}>
        <CssBaseline />
        <Container maxWidth="sm" sx={{ minHeight: '100vh', display: 'flex', flexDirection: 'column', justifyContent: 'center', textAlign: 'center' }}>
          <Typography variant="h5" color="success.main" gutterBottom>Запись подтверждена!</Typography>
          <Typography variant="subtitle1" color="text.secondary" gutterBottom>
            Спасибо за запись! Мы ждём вас в клинике.
          </Typography>
          <Button variant="contained" color="primary" sx={{ borderRadius: 3, mt: 3 }} onClick={() => {
            setStep('main');
            setSelectedService(null);
            setSelectedDate(null);
            setSelectedTime(null);
            setPhone('');
            setSmsSent(false);
            setSmsCode('');
            setSmsError('');
          }}>В главное меню</Button>
        </Container>
      </ThemeProvider>
    );
  }

  return null;
}

export default App; 