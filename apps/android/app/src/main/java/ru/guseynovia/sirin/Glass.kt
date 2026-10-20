package ru.guseynovia.sirin

import androidx.compose.animation.core.animateDpAsState
import androidx.compose.animation.core.animateFloatAsState
import androidx.compose.animation.core.spring
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.interaction.collectIsPressedAsState
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxScope
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.composed
import androidx.compose.ui.draw.clip
import androidx.compose.ui.draw.shadow
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

/**
 * Стилистика «Крипто-защита» (Dark Material & Cyber-Neon).
 */
object Glass {
    // Основной фон: Глубокий темный #0B0C10
    val bgTop = Color(0xFF0B0C10)
    val bgBottom = Color(0xFF08090D)
    val glowCyan = Color(0x1800FFFF)
    val glowPurple = Color(0x188A2BE2)

    // Карточки и плашки: #151623, скругленные углы 14dp (~12-16dp)
    val cardFill = Color(0xFF151623)
    val cardBorder = Color(0xFF222538)
    val cardBorderAccent = Brush.linearGradient(listOf(Brand.cyberBlue, Brand.neonPurple))

    val pillFillTop = Color(0xFF1B1D2D)
    val pillFillBottom = Color(0xFF10121C)
    val pillBorder = Color(0xFF2E334D)

    val cardShape = RoundedCornerShape(14.dp)
    val pillShape = RoundedCornerShape(50)
}

/** Фон приложения: Глубокий темный #0B0C10 с мягкими кибер-подсветками по краям. */
@Composable
fun GlassBackground(content: @Composable BoxScope.() -> Unit) {
    Box(
        modifier = Modifier
            .fillMaxSize()
            .background(Brush.verticalGradient(listOf(Glass.bgTop, Glass.bgBottom)))
            .background(
                Brush.radialGradient(
                    listOf(Glass.glowCyan, Color.Transparent),
                    center = Offset(0f, 0f),
                    radius = 1200f,
                )
            )
            .background(
                Brush.radialGradient(
                    listOf(Glass.glowPurple, Color.Transparent),
                    center = Offset(1200f, 1000f),
                    radius = 1200f,
                )
            ),
        content = content,
    )
}

/** Стилизация карточки: темный фон #151623, скругление ~14dp, опциональная градиентная обводка (1px stroke). */
fun Modifier.glassCard(accentBorder: Boolean = false): Modifier = this
    .clip(Glass.cardShape)
    .background(Glass.cardFill)
    .then(
        if (accentBorder) {
            Modifier.border(1.dp, Glass.cardBorderAccent, Glass.cardShape)
        } else {
            Modifier.border(1.dp, Glass.cardBorder, Glass.cardShape)
        }
    )

/** Кнопка-таблетка в кибер-стиле. */
@Composable
fun GlassButton(
    text: String,
    modifier: Modifier = Modifier,
    enabled: Boolean = true,
    height: Dp = 48.dp,
    onClick: () -> Unit,
) {
    Box(
        modifier = modifier
            .glassPress(enabled = enabled) { onClick() }
            .height(height)
            .clip(Glass.pillShape)
            .background(Color(0xFF181B2B))
            .border(1.dp, if (enabled) Glass.pillBorder else Color(0xFF1E2133), Glass.pillShape)
            .padding(horizontal = 18.dp),
        contentAlignment = Alignment.Center,
    ) {
        Text(
            text,
            color = if (enabled) Brand.text else Brand.dim,
            fontSize = 15.sp,
            fontWeight = androidx.compose.ui.text.font.FontWeight.SemiBold,
        )
    }
}

/**
 * Сегментный переключатель в кибер-стиле.
 */
@Composable
fun GlassSegmented(
    options: List<String>,
    selected: Int,
    modifier: Modifier = Modifier,
    enabled: Boolean = true,
    onSelect: (Int) -> Unit,
) {
    Row(
        modifier = modifier
            .height(44.dp)
            .clip(Glass.pillShape)
            .background(Color(0xFF0F1019))
            .border(1.dp, Glass.cardBorder, Glass.pillShape)
            .padding(3.dp),
    ) {
        options.forEachIndexed { i, title ->
            val on = i == selected
            Box(
                modifier = Modifier
                    .weight(1f)
                    .fillMaxHeight()
                    .glassPress(enabled = enabled) { onSelect(i) }
                    .clip(Glass.pillShape)
                    .then(
                        if (on) Modifier
                            .background(Brush.horizontalGradient(listOf(Color(0x3300FFFF), Color(0x338A2BE2))))
                            .border(1.dp, Brush.horizontalGradient(listOf(Brand.cyberBlue, Brand.neonPurple)), Glass.pillShape)
                        else Modifier
                    ),
                contentAlignment = Alignment.Center,
            ) {
                Text(
                    title,
                    color = if (on) Brand.cyberBlue else Brand.dim,
                    fontSize = 14.sp,
                    fontWeight = if (on) androidx.compose.ui.text.font.FontWeight.SemiBold else androidx.compose.ui.text.font.FontWeight.Normal,
                )
            }
        }
    }
}

/** Карточка на всю ширину с внутренним отступом. */
@Composable
fun GlassCard(
    modifier: Modifier = Modifier,
    accentBorder: Boolean = false,
    content: @Composable () -> Unit
) {
    Box(modifier = modifier.fillMaxWidth().glassCard(accentBorder = accentBorder).padding(16.dp)) { content() }
}

/** Круглая кнопка-значок (назад, настройки, копировать). */
fun Modifier.glassCircle(): Modifier = this
    .clip(CircleShape)
    .background(Color(0xFF151828))
    .border(1.dp, Color(0xFF282D47), CircleShape)

/** Объём и градиенты элементов */
object GlassVolume {
    val fill = Brush.verticalGradient(
        0f to Color(0x2800FFFF),
        1f to Color(0x188A2BE2),
    )
    val border = Brush.verticalGradient(
        0f to Color(0x6600FFFF),
        1f to Color(0x338A2BE2),
    )
    val cardFill = Brush.verticalGradient(
        0f to Color(0xFF17192A),
        1f to Color(0xFF121422),
    )
    val cardBorder = Brush.verticalGradient(
        0f to Color(0xFF2B314F),
        1f to Color(0xFF1A1C2C),
    )
}

/**
 * Анимированное нажатие.
 */
fun Modifier.glassPress(
    enabled: Boolean = true,
    onClickLabel: String? = null,
    onClick: () -> Unit,
): Modifier = composed {
    val source = remember { MutableInteractionSource() }
    val pressed by source.collectIsPressedAsState()
    val scale by animateFloatAsState(
        targetValue = if (pressed && enabled) 0.95f else 1f,
        animationSpec = spring(dampingRatio = 0.6f, stiffness = 700f),
        label = "press",
    )
    this
        .graphicsLayer { scaleX = scale; scaleY = scale }
        .clickable(
            interactionSource = source,
            indication = null,
            enabled = enabled,
            onClickLabel = onClickLabel,
        ) { onClick() }
}

/**
 * Акцентная кнопка-таблетка с кибер-градиентом.
 */
@Composable
fun GlassAccentButton(
    text: String,
    modifier: Modifier = Modifier,
    enabled: Boolean = true,
    height: Dp = 48.dp,
    onClick: () -> Unit,
) {
    Box(
        modifier = modifier
            .glassPress(enabled = enabled) { onClick() }
            .height(height)
            .clip(Glass.pillShape)
            .background(
                if (enabled) {
                    Brush.horizontalGradient(listOf(Brand.cyberBlue, Brand.neonPurple))
                } else {
                    Brush.horizontalGradient(listOf(Color(0xFF1F2538), Color(0xFF151825)))
                }
            )
            .padding(horizontal = 20.dp),
        contentAlignment = Alignment.Center,
    ) {
        Text(
            text,
            color = if (enabled) Color.Black else Brand.dim,
            fontSize = 15.sp,
            fontWeight = androidx.compose.ui.text.font.FontWeight.Bold,
        )
    }
}

/**
 * Свитч по ТЗ (Settings Toggle):
 * - ВЫКЛ: Стандартный серый ползунок, темный трек без яркой обводки.
 * - ВКЛ: Ползунок переезжает вправо, заливается сине-пурпурным градиентом с тенью/свечением.
 */
@Composable
fun GlassSwitch(
    checked: Boolean,
    onCheckedChange: (Boolean) -> Unit,
    modifier: Modifier = Modifier,
    enabled: Boolean = true,
) {
    val offset by animateDpAsState(
        targetValue = if (checked) 24.dp else 3.dp,
        animationSpec = spring(dampingRatio = 0.7f, stiffness = 600f),
        label = "switch",
    )

    Box(
        modifier = modifier
            .glassPress(enabled = enabled) { onCheckedChange(!checked) }
            .size(width = 54.dp, height = 32.dp)
            .clip(Glass.pillShape)
            .background(
                if (checked) {
                    Brush.horizontalGradient(listOf(Color(0x3300FFFF), Color(0x338A2BE2)))
                } else {
                    Brush.verticalGradient(listOf(Color(0xFF131520), Color(0xFF0F1018)))
                }
            )
            .then(
                if (checked) {
                    Modifier.border(1.dp, Brush.horizontalGradient(listOf(Brand.cyberBlue, Brand.neonPurple)), Glass.pillShape)
                } else {
                    Modifier.border(1.dp, Color(0xFF222538), Glass.pillShape)
                }
            ),
        contentAlignment = Alignment.CenterStart
    ) {
        // Ползунок (thumb)
        Box(
            modifier = Modifier
                .padding(start = offset)
                .size(24.dp)
                .shadow(
                    elevation = if (checked) 6.dp else 2.dp,
                    shape = CircleShape,
                    ambientColor = if (checked) Brand.cyberBlue else Color.Transparent,
                    spotColor = if (checked) Brand.neonPurple else Color.Transparent
                )
                .clip(CircleShape)
                .background(
                    if (checked) {
                        Brush.linearGradient(listOf(Brand.cyberBlue, Brand.neonPurple))
                    } else {
                        Brush.verticalGradient(listOf(Color(0xFF8A93A8), Color(0xFF6B7285)))
                    }
                )
        )
    }
}

/**
 * Кибер-галочка (Checkbox).
 */
@Composable
fun GlassCheck(
    checked: Boolean,
    onCheckedChange: (Boolean) -> Unit,
    modifier: Modifier = Modifier,
) {
    val shape = RoundedCornerShape(8.dp)
    androidx.compose.foundation.Canvas(
        modifier = modifier
            .glassPress { onCheckedChange(!checked) }
            .size(26.dp)
            .clip(shape)
            .background(
                if (checked) {
                    Brush.linearGradient(listOf(Brand.cyberBlue, Brand.neonPurple))
                } else {
                    Brush.verticalGradient(listOf(Color(0xFF181B2B), Color(0xFF11131E)))
                }
            )
            .border(
                1.dp,
                if (checked) Brush.linearGradient(listOf(Brand.cyberBlue, Brand.neonPurple)) else Brush.verticalGradient(listOf(Color(0xFF2C324E), Color(0xFF1F2338))),
                shape
            ),
    ) {
        if (checked) {
            val w = size.minDimension
            val p = androidx.compose.ui.graphics.Path().apply {
                moveTo(w * 0.26f, w * 0.52f)
                lineTo(w * 0.43f, w * 0.69f)
                lineTo(w * 0.75f, w * 0.33f)
            }
            drawPath(
                p, Color.Black,
                style = androidx.compose.ui.graphics.drawscope.Stroke(
                    width = w * 0.12f,
                    cap = androidx.compose.ui.graphics.StrokeCap.Round,
                    join = androidx.compose.ui.graphics.StrokeJoin.Round,
                ),
            )
        }
    }
}
