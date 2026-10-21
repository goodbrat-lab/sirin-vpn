package ru.guseynovia.sirin

import androidx.compose.foundation.clickable
import androidx.compose.foundation.verticalScroll
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import android.content.Intent
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.Button
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.OutlinedTextField
import androidx.compose.runtime.rememberCoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import androidx.compose.material3.Switch
import androidx.compose.material3.TextButton
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalContext
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

/**
 * Настройки: два входа и больше ничего.
 *
 * Всё, что человеку не нужно каждый день, живёт здесь. На главном
 * экране осталась планета, отсчёт и доступ — то, ради чего приложение
 * открывают.
 */
@Composable
fun SettingsScreen(
    modifier: Modifier = Modifier,
    onHashes: () -> Unit,
    onLogs: () -> Unit,
    onSplit: () -> Unit,
    onBack: () -> Unit,
    onConnect: () -> Unit = {},
) {
    val context = LocalContext.current
    var showChangeKeyDialog by remember { mutableStateOf(false) }

    Column(
        // ПРОКРУТКА (03.10): отладочные строки вытеснили строку версии за
        // нижний край, и обновиться было нечем.
        modifier = modifier.fillMaxSize()
            .verticalScroll(androidx.compose.foundation.rememberScrollState())
            .padding(16.dp),
        verticalArrangement = Arrangement.spacedBy(4.dp),
    ) {
        Row(verticalAlignment = Alignment.CenterVertically) {
            BackButton(onClick = onBack)
            Text("Настройки", color = Brand.text, fontSize = 20.sp)
        }

        // ПОРЯДОК ОТ ЧАСТОГО К РЕДКОМУ.
        //
        // Раздельное туннелирование настраивают чаще всего: банк мимо
        // VPN, потом ещё одно приложение, потом ещё. Обход белых
        // списков — раз в несколько дней, когда кончаются ссылки.
        // Логи — только когда что-то сломалось, то есть почти никогда.
        Column(modifier = Modifier.fillMaxWidth().padding(top = 8.dp).glassCard().padding(horizontal = 16.dp)) {
        Entry(
            title = "Раздельное туннелирование",
            hint = SplitTunnel.title(SplitTunnel.mode.value).lowercase() +
                ", выбрано " + SplitTunnel.chosen.value.size,
            icon = { AppsIcon() },
            onClick = onSplit,
        )
        Entry(
            title = "Сменить ключ",
            hint = "ввести новый UUID",
            icon = { KeyIcon() },
            onClick = { showChangeKeyDialog = true },
        )
        Entry(
            title = "Обход белых списков",
            // Слова «релей» здесь нет намеренно: подпись видна в
            // приложении и попадает в пакет, а 61.5 такие слова
            // запрещает. Смысл для человека при этом не теряется.
            hint = "ссылки на звонки, через них работает запасной путь",
            icon = { LinkIcon() },
            onClick = onHashes,
        )
        Entry(
            title = "Логи",
            hint = "что происходило внутри",
            icon = { LogIcon() },
            onClick = onLogs,
        )
        }

        // Срок доступа И остаток пробного периода — ЗДЕСЬ, внизу
        // настроек, а не на главном экране (55.3 + просьба владельца):
        // смотрят на них изредка, а место на главном они занимали
        // всегда. Планета сама показывает, что доступ есть.
        Spacer(modifier = Modifier.padding(top = 24.dp))
        // Пробный период называем пробным, даже когда он пришёл серверным
        // ключом с датой (тогда state=KEYED): иначе рядом с «Доступ до …»
        // появляется «Купить», и непонятно, почему предлагают купить уже
        // выданный доступ. Оплаченный ключ — обычный keyText.
        val bottomAccess = when {
            Access.onTrial() && Access.state.value == Access.State.KEYED ->
                "Пробный период — " + Access.keyText().replaceFirstChar { it.lowercase() }
            Access.state.value == Access.State.KEYED -> Access.keyText()
            Access.state.value == Access.State.TRIAL ->
                "Пробный период: осталось " + Access.leftText()
            else -> ""
        }
        if (bottomAccess.isNotEmpty()) {
            Text(
                bottomAccess,
                color = Brand.text,
                fontSize = 14.sp,
                modifier = Modifier.fillMaxWidth().padding(bottom = 8.dp),
                textAlign = TextAlign.Center,
            )
        }

        // ВО ВРЕМЯ ТРИАЛА — покупка и ввод ключа ЗДЕСЬ, рядом с остатком
        // (просьба владельца 13.09): дать купить/ввести ключ, не дожидаясь
        // конца пробного периода. На главном экране в триале их нет; как
        // только триал кончится, state станет OVER и предложение вернётся
        // на главный само. В остальных состояниях блок доступа живёт на
        // главном (AccessBlock), здесь его дублировать не надо.
        if (Access.onTrial()) {
            AccessOffer(onConnect = onConnect)
        }

        // Отладочные настройки. В релизе и в сборке для Play — пусто, и
        // не веткой, а отсутствием кода: см. DebugSettings.kt в наборах
        // directDebug и directRelease.
        DebugRows()

        // Блок автообновления клиента Sirin через GitHub Releases
        Spacer(modifier = Modifier.padding(top = 16.dp))
        Column(
            modifier = Modifier
                .fillMaxWidth()
                .glassCard()
                .padding(16.dp),
            verticalArrangement = Arrangement.spacedBy(10.dp)
        ) {
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.SpaceBetween,
                verticalAlignment = Alignment.CenterVertically
            ) {
                Column {
                    Text(
                        text = "Версия: ${appVersion(context)}",
                        color = Brand.text,
                        fontSize = 14.sp,
                        fontWeight = androidx.compose.ui.text.font.FontWeight.Medium
                    )
                    val statusText = when (val st = UpdateChecker.status.value) {
                        is UpdateChecker.CheckStatus.Idle -> "Нажмите для проверки"
                        is UpdateChecker.CheckStatus.Checking -> "Проверка обновлений…"
                        is UpdateChecker.CheckStatus.UpToDate -> "Обновлений нет (установлена последняя)"
                        is UpdateChecker.CheckStatus.Available -> "Доступна v${st.release.versionName}"
                        is UpdateChecker.CheckStatus.Downloading -> "Скачивание: ${(st.progress * 100).toInt()}% (${st.currentMb.toInt()} из ${st.totalMb.toInt()} МБ)"
                        is UpdateChecker.CheckStatus.ReadyToInstall -> "Загрузка завершена. Готово к установке."
                        is UpdateChecker.CheckStatus.Error -> "Ошибка сети: ${st.message}"
                    }
                    val statusColor = when (UpdateChecker.status.value) {
                        is UpdateChecker.CheckStatus.Available -> Brand.cyberBlue
                        is UpdateChecker.CheckStatus.Error -> Brand.neonRed
                        is UpdateChecker.CheckStatus.ReadyToInstall -> Brand.neonOrange
                        else -> Brand.dim
                    }
                    Text(
                        text = statusText,
                        color = statusColor,
                        fontSize = 12.sp
                    )
                }
            }

            when (val st = UpdateChecker.status.value) {
                is UpdateChecker.CheckStatus.Available -> {
                    GlassAccentButton(
                        text = "Скачать v${st.release.versionName}",
                        modifier = Modifier.fillMaxWidth(),
                        onClick = { UpdateChecker.startDownloadAndInstall(context, st.release) }
                    )
                }
                is UpdateChecker.CheckStatus.ReadyToInstall -> {
                    GlassAccentButton(
                        text = "Установить обновление",
                        modifier = Modifier.fillMaxWidth(),
                        onClick = { UpdateChecker.installApk(context, st.apkFile) }
                    )
                }
                is UpdateChecker.CheckStatus.Downloading -> {
                    androidx.compose.material3.LinearProgressIndicator(
                        progress = { st.progress },
                        modifier = Modifier.fillMaxWidth(),
                        color = Brand.cyberBlue,
                        trackColor = Brand.space
                    )
                }
                else -> {
                    GlassButton(
                        text = "Проверить обновления",
                        modifier = Modifier.fillMaxWidth(),
                        onClick = { UpdateChecker.checkManually(context) }
                    )
                }
            }
        }
    }

    if (showChangeKeyDialog) {
        var newKey by remember { mutableStateOf("") }
        AlertDialog(
            onDismissRequest = { showChangeKeyDialog = false },
            title = { Text("Сменить ключ") },
            text = {
                Column {
                    Text("Введите новый UUID (получить можно в @sirin_crp_bot):")
                    OutlinedTextField(
                        value = newKey,
                        onValueChange = { newKey = it.trim() },
                        placeholder = { Text("xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx") },
                        singleLine = true,
                        modifier = Modifier.fillMaxWidth().padding(top = 8.dp),
                    )
                    if (newKey.isNotEmpty() && newKey.length != 36) {
                        Text(
                            "UUID должен быть 36 символов",
                            color = Color.Red,
                            fontSize = 12.sp,
                            modifier = Modifier.padding(top = 4.dp),
                        )
                    }
                }
            },
            confirmButton = {
                TextButton(
                    enabled = newKey.length == 36,
                    onClick = {
                        Access.acceptKey(newKey)
                        showChangeKeyDialog = false
                        TunnelLog.add("ключ сменён через настройки")
                        if (TunnelState.connected.value) {
                            context.startService(
                                Intent(context, SirinVpnService::class.java)
                                    .setAction(SirinVpnService.ACTION_RECONNECT)
                            )
                        }
                    },
                ) {
                    Text("Сохранить")
                }
            },
            dismissButton = {
                TextButton(onClick = { showChangeKeyDialog = false }) {
                    Text("Отмена")
                }
            },
        )
    }
}

/**
 * Строка-вход.
 *
 * ПОДПИСЬ ЗДЕСЬ ОБЯЗАТЕЛЬНА, и это не нарушение правила из 51.6.
 * Правило про кнопки действия: значок «поделиться» или «удалить»
 * понятен без слов. А это навигация — значок цепи не скажет, что за
 * ним ссылки на звонки VK, а не что-нибудь ещё. Пункт меню без имени
 * это загадка, а не оформление.
 */
@Composable
private fun Entry(
    title: String,
    hint: String,
    icon: @Composable () -> Unit,
    onClick: () -> Unit,
) {
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .clickable { onClick() }
            .padding(vertical = 14.dp),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(12.dp),
    ) {
        icon()
        Column {
            Text(title, color = Brand.text, fontSize = 17.sp)
            Text(hint, color = Brand.dim, fontSize = 13.sp)
        }
    }
    HorizontalDivider(color = Glass.cardBorder)
}

/**
 * Логи. Раньше жили на главном экране и занимали его целиком.
 */
@Composable
fun LogScreen(
    modifier: Modifier = Modifier,
    onBack: () -> Unit,
    onShare: () -> Unit,
    onCopy: () -> String = { "" },
) {
    var copied by androidx.compose.runtime.remember { androidx.compose.runtime.mutableStateOf("") }
    Column(
        modifier = modifier.fillMaxSize().padding(horizontal = 12.dp),
        verticalArrangement = Arrangement.spacedBy(4.dp),
    ) {
        Row(
            modifier = Modifier.fillMaxWidth(),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            BackButton(onClick = onBack)
            Text(
                "Логи",
                color = Brand.text,
                fontSize = 20.sp,
                modifier = Modifier.weight(1f),
            )
            // Копировать в буфер — значок «два листа», поделиться (сохранить
            // файлом, отправить) — стрелка «наружу». Отправки в поддержку
            // из приложения больше нет (решение владельца 01.10).
            CopyButton(onClick = { copied = onCopy() })
            androidx.compose.foundation.layout.Spacer(modifier = Modifier.padding(horizontal = 4.dp))
            // Значок без подписи: стрелка «наружу» — общепринятое
            // «поделиться», объяснять её нечем.
            ShareButton(onClick = onShare)
        }

        if (copied.isNotEmpty()) {
            Text(copied, color = Brand.dim, fontSize = 13.sp)
        }

        LazyColumn(modifier = Modifier.weight(1f).fillMaxWidth()) {
            items(TunnelLog.lines) { line ->
                Text(text = line, color = Brand.text, fontSize = 12.sp)
            }
        }
    }
}

