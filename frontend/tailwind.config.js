/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{vue,js,ts,jsx,tsx}'],
  darkMode: 'class',
  theme: {
    extend: {
      colors: {
        // Terra primary - forest green
        primary: {
          50: '#f3f7ef',
          100: '#e4eddd',
          200: '#ccdcbc',
          300: '#abc595',
          400: '#7fa36f',
          500: '#4a7c59',
          600: '#3f6a4d',
          700: '#345740',
          800: '#2a4635',
          900: '#22392d',
          950: '#111f17'
        },
        // Terra accent - warm amber/earth
        accent: {
          50: '#fbf7ee',
          100: '#f4ead6',
          200: '#e7d2a9',
          300: '#d6b574',
          400: '#bb9146',
          500: '#705c30',
          600: '#5e4c28',
          700: '#4c3e23',
          800: '#3d321f',
          900: '#332b1d',
          950: '#1d160d'
        },
        terra: {
          bg: '#faf6f0',
          surface: '#f5f1ea',
          sunken: '#f0ece4',
          elevated: '#fffaf4',
          line: '#ded5c6',
          ink: '#2e3230',
          muted: '#6f746a',
          amber: '#705c30'
        },
        // Warm dark mode
        dark: {
          50: '#f7f4ed',
          100: '#ece6da',
          200: '#d5cabb',
          300: '#b6aa98',
          400: '#8d8374',
          500: '#6f675b',
          600: '#575246',
          700: '#3d463c',
          800: '#273027',
          900: '#1a211b',
          950: '#101511'
        }
      },
      opacity: {
        86: '0.86',
        88: '0.88'
      },
      fontFamily: {
        display: [
          'Literata',
          'Georgia',
          'Songti SC',
          'STSong',
          'serif'
        ],
        sans: [
          'Nunito Sans',
          'PingFang SC',
          'Hiragino Sans GB',
          'Microsoft YaHei',
          'system-ui',
          '-apple-system',
          'BlinkMacSystemFont',
          'Segoe UI',
          'sans-serif'
        ],
        mono: ['ui-monospace', 'SFMono-Regular', 'Menlo', 'Monaco', 'Consolas', 'monospace']
      },
      boxShadow: {
        glass: '0 4px 20px rgba(46, 50, 48, 0.06)',
        'glass-sm': '0 2px 12px rgba(46, 50, 48, 0.05)',
        glow: '0 0 0 4px rgba(74, 124, 89, 0.12)',
        'glow-lg': '0 8px 30px rgba(74, 124, 89, 0.16)',
        card: '0 4px 20px rgba(46, 50, 48, 0.06)',
        'card-hover': '0 10px 32px rgba(46, 50, 48, 0.09)',
        'inner-glow': 'inset 0 1px 0 rgba(255, 250, 244, 0.6)'
      },
      backgroundImage: {
        'gradient-radial': 'radial-gradient(var(--tw-gradient-stops))',
        'gradient-primary': 'linear-gradient(135deg, #4a7c59 0%, #3f6a4d 100%)',
        'gradient-dark': 'linear-gradient(135deg, #273027 0%, #101511 100%)',
        'gradient-glass':
          'linear-gradient(135deg, rgba(255,250,244,0.82) 0%, rgba(245,241,234,0.72) 100%)',
        'mesh-gradient':
          'linear-gradient(180deg, #faf6f0 0%, #f5f1ea 100%)'
      },
      animation: {
        'fade-in': 'fadeIn 0.3s ease-out',
        'slide-up': 'slideUp 0.3s ease-out',
        'slide-down': 'slideDown 0.3s ease-out',
        'slide-in-right': 'slideInRight 0.3s ease-out',
        'scale-in': 'scaleIn 0.2s ease-out',
        'pulse-slow': 'pulse 3s cubic-bezier(0.4, 0, 0.6, 1) infinite',
        shimmer: 'shimmer 2s linear infinite',
        glow: 'glow 2s ease-in-out infinite alternate'
      },
      keyframes: {
        fadeIn: {
          '0%': { opacity: '0' },
          '100%': { opacity: '1' }
        },
        slideUp: {
          '0%': { opacity: '0', transform: 'translateY(10px)' },
          '100%': { opacity: '1', transform: 'translateY(0)' }
        },
        slideDown: {
          '0%': { opacity: '0', transform: 'translateY(-10px)' },
          '100%': { opacity: '1', transform: 'translateY(0)' }
        },
        slideInRight: {
          '0%': { opacity: '0', transform: 'translateX(20px)' },
          '100%': { opacity: '1', transform: 'translateX(0)' }
        },
        scaleIn: {
          '0%': { opacity: '0', transform: 'scale(0.95)' },
          '100%': { opacity: '1', transform: 'scale(1)' }
        },
        shimmer: {
          '0%': { backgroundPosition: '-200% 0' },
          '100%': { backgroundPosition: '200% 0' }
        },
        glow: {
          '0%': { boxShadow: '0 0 0 4px rgba(74, 124, 89, 0.10)' },
          '100%': { boxShadow: '0 0 0 6px rgba(74, 124, 89, 0.16)' }
        }
      },
      backdropBlur: {
        xs: '2px'
      },
      borderRadius: {
        '4xl': '2rem'
      }
    }
  },
  plugins: []
}
