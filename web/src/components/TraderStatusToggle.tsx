import { useState } from 'react';
import { api } from '../lib/api';
import { useLanguage } from '../contexts/LanguageContext';
import { t } from '../i18n/translations';

interface TraderStatusToggleProps {
  traderId: string;
  isRunning: boolean;
  onToggle: (newStatus: boolean) => void;
}

export function TraderStatusToggle({ traderId, isRunning, onToggle }: TraderStatusToggleProps) {
  const { language } = useLanguage();
  const [isLoading, setIsLoading] = useState(false);

  const handleToggle = async () => {
    if (!traderId) return;

    setIsLoading(true);
    try {
      if (isRunning) {
        await api.stopTrader(traderId);
      } else {
        await api.startTrader(traderId);
      }
      onToggle(!isRunning); // Notify parent component of the change
    } catch (error) {
      console.error("Failed to toggle trader status:", error);
      alert(t('toggleError', language)); // Show a simple alert for now
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <div className="flex items-center gap-2">
      <button
        onClick={handleToggle}
        disabled={isLoading}
        className={`relative inline-flex h-6 w-11 items-center rounded-full transition-colors focus:outline-none focus:ring-2 focus:ring-offset-2
          ${isRunning ? 'bg-green-500' : 'bg-gray-600'}
          ${isLoading ? 'opacity-50 cursor-not-allowed' : ''}
        `}
      >
        <span
          className={`inline-block h-4 w-4 transform rounded-full bg-white transition-transform
            ${isRunning ? 'translate-x-6' : 'translate-x-1'}
          `}
        />
      </button>
      <span className="font-semibold mono text-xs" style={{ color: isRunning ? '#0ECB81' : '#F6465D' }}>
        {isLoading ? t('loading', language) : t(isRunning ? 'running' : 'stopped', language)}
      </span>
    </div>
  );
}
