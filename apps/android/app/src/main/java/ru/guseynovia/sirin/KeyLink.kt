package ru.guseynovia.sirin

import android.content.Intent
import androidx.compose.runtime.mutableStateOf

/**
 * Ключ по ссылке из бота: sirin://key/<ключ> (просьба владельца 25.09).
 *
 * Человек нажимает ссылку в Telegram — открывается приложение, и ключ уже
 * подставлен: самый частый шаг, на котором ошибаются (скопировать ключ
 * целиком, без пробела и обрезки), исчезает.
 *
 * ПРИМЕНЯЕТСЯ ТОЛЬКО ПОСЛЕ НАЖАТИЯ «ПРИМЕНИТЬ». Ссылку может прислать кто
 * угодно, и молча подменить ключ по чужой ссылке нельзя: на экране
 * показываем последние знаки ключа и спрашиваем. Дальше тот же путь, что у
 * ручного ввода: Api.checkKey → applyVerdict (AccessScreen.KeyLinkPrompt).
 *
 * ТОЛЬКО ПРЯМАЯ РАЗДАЧА. Фильтр ссылки объявлен в
 * app/src/direct/AndroidManifest.xml, а здесь дополнительно отсекается
 * KEY_ENTRY: в сборке для Play ключ, купленный мимо магазина, вводить
 * нельзя по его правилам — и по ссылке тоже.
 *
 * КЛЮЧ НЕ ПИШЕТСЯ В ЛОГ НИКОГДА — ни целиком, ни частью.
 */
object KeyLink {

    /** Ключ из последней ссылки или сканера, ждущий подтверждения. Пусто — ничего. */
    val pending = mutableStateOf("")

    /**
     * Извлечь и нормализовать ключ из любой строки:
     * - sirin://key/<ключ>
     * - sirin://key?uuid=<ключ>
     * - https://crp.guseynovia.ru/key?uuid=<ключ>
     * - сырой UUID/ключ (без пробелов, длина >= 8)
     */
    fun extractKey(rawInput: String?): String? {
        if (rawInput.isNullOrBlank()) return null
        val s = rawInput.trim()
        if (s.startsWith("sirin://", ignoreCase = true) || s.startsWith("https://", ignoreCase = true)) {
            val uri = try { android.net.Uri.parse(s) } catch (_: Throwable) { null }
            if (uri != null) {
                val candidate = uri.getQueryParameter("uuid")
                    ?: uri.getQueryParameter("k")
                    ?: uri.lastPathSegment
                if (!candidate.isNullOrBlank() && candidate.trim().length >= 8) {
                    return candidate.trim()
                }
            }
        }
        val cleaned = s.removePrefix("sirin://").removePrefix("sirin:").removePrefix("key:").trim()
        if (cleaned.length >= 8 && !cleaned.contains(' ') && !cleaned.contains('\n') && !cleaned.contains('\r')) {
            return cleaned
        }
        return null
    }

    /** Разобрать интент активности. Чужие интенты игнорирует. */
    fun take(intent: Intent?) {
        if (!KEY_ENTRY) return
        val uri = intent?.data ?: return
        val isSirinScheme = uri.scheme == "sirin" && uri.host == "key"
        val isHttpsLink = uri.scheme == "https" && uri.host == "crp.guseynovia.ru" && (uri.path?.startsWith("/key") == true)
        if (!isSirinScheme && !isHttpsLink) return
        val k = extractKey(uri.toString()) ?: return
        if (k.length < 8) {
            TunnelLog.add("ссылка с ключом: ключ слишком короткий — пропускаю")
            return
        }
        pending.value = k
        Notice.clear()
        TunnelLog.add("ссылка с ключом получена, жду подтверждения")
    }

    /** Принять ключ напрямую из QR-сканера или буфера обмена. */
    fun takeRaw(rawInput: String?): Boolean {
        if (!KEY_ENTRY) return false
        val k = extractKey(rawInput) ?: return false
        pending.value = k
        Notice.clear()
        TunnelLog.add("ключ получен из сканера/буфера, жду подтверждения")
        return true
    }

    /** Показ на экране: только последние четыре знака, чтобы узнать ключ, не раскрывая его. */
    fun masked(k: String): String =
        "…" + k.takeLast(4)

    fun clear() {
        pending.value = ""
    }
}
