import { PencilSimpleIcon } from '@phosphor-icons/react';
import {
  CHANNEL_PRESETS,
  PRESET_CATEGORY_LABELS,
  type ChannelPreset,
  type PresetCategory,
} from '../lib/channelPresets';
import { ChannelIcon } from './ModelIcon';

const CATEGORY_ORDER: PresetCategory[] = ['global', 'cn', 'aggregator'];

interface Props {
  /** Selected preset id, or 'custom'. */
  value: string;
  onSelect: (preset: ChannelPreset | null) => void;
}

function PresetButton({
  selected,
  onClick,
  children,
  label,
}: {
  selected: boolean;
  onClick: () => void;
  children: React.ReactNode;
  label: string;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      aria-pressed={selected}
      title={label}
      className={`flex items-center gap-1.5 rounded-xl border px-2.5 py-2 text-left text-xs font-bold transition ${
        selected
          ? 'border-sakura-400 bg-sakura-50 text-ink ring-2 ring-sakura-100 dark:bg-sakura-500/15 dark:ring-sakura-500/20'
          : 'border-sakura-100 bg-surface text-ink-soft hover:border-sakura-300 hover:text-ink dark:border-white/10'
      }`}
    >
      {children}
      <span className="min-w-0 truncate">{label}</span>
    </button>
  );
}

// Icon-grid replacement for the native preset <select>: presets grouped by
// category, plus a leading "custom" entry that clears the pre-filled values.
export function ChannelPresetPicker({ value, onSelect }: Props) {
  return (
    <div className="flex flex-col gap-3 rounded-2xl border border-sakura-100 bg-sakura-50/40 p-3 dark:border-white/10 dark:bg-sakura-500/5">
      <div className="grid grid-cols-2 gap-1.5 sm:grid-cols-3">
        <PresetButton selected={value === 'custom'} onClick={() => onSelect(null)} label="自定义…">
          <span className="inline-flex shrink-0 items-center text-sakura-400" aria-hidden>
            <PencilSimpleIcon size={16} weight="duotone" />
          </span>
        </PresetButton>
      </div>
      {CATEGORY_ORDER.map((category) => (
        <div key={category}>
          <div className="mb-1.5 text-[11px] font-bold uppercase tracking-wide text-ink-soft">
            {PRESET_CATEGORY_LABELS[category]}
          </div>
          <div className="grid grid-cols-2 gap-1.5 sm:grid-cols-3">
            {CHANNEL_PRESETS.filter((p) => p.category === category).map((p) => (
              <PresetButton
                key={p.id}
                selected={value === p.id}
                onClick={() => onSelect(p)}
                label={p.label}
              >
                <ChannelIcon icon={p.icon} type={p.type} size={16} />
              </PresetButton>
            ))}
          </div>
        </div>
      ))}
    </div>
  );
}
