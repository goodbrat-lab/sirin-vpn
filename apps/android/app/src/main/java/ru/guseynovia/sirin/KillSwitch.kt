package ru.guseynovia.sirin

import android.content.Context
import androidx.compose.runtime.mutableStateOf

/**
 * Kill Switch: блокировка трафика при обрыве туннеля для предотвращения
 * утечки реального IP-адреса пользователя.
 *
 * По умолчанию включён (true). При обрыве соединения туннель не отдаёт
 * трафик в открытую сеть оператора, а удерживает blackhole-заглушку
 * до успешного восстановления туннеля.
 */
object KillSwitch {
    private const val PREFS = "sirin_settings"
    private const val KEY = "kill_switch"

    val enabled = mutableStateOf(true)

    fun attach(ctx: Context) {
        val prefs = ctx.getSharedPreferences(PREFS, Context.MODE_PRIVATE)
        enabled.value = prefs.getBoolean(KEY, true)
    }

    fun set(ctx: Context, on: Boolean) {
        enabled.value = on
        ctx.getSharedPreferences(PREFS, Context.MODE_PRIVATE)
            .edit()
            .putBoolean(KEY, on)
            .apply()
        TunnelLog.add("Kill Switch: ${if (on) "включён" else "выключен"}")
    }
}
