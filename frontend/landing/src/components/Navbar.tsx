import { useState } from 'react'
import { Menu, X, Droplets } from 'lucide-react'

export default function Navbar() {
  const [open, setOpen] = useState(false)

  return (
    <nav className="fixed top-0 left-0 right-0 z-50 bg-white/95 backdrop-blur-sm border-b border-gray-100 shadow-sm">
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
        <div className="flex items-center justify-between h-16">
          {/* Logo */}
          <div className="flex items-center gap-2">
            <div className="w-8 h-8 bg-green-800 rounded-lg flex items-center justify-center">
              <Droplets className="w-5 h-5 text-yellow-400" />
            </div>
            <div>
              <span className="font-bold text-green-900 text-lg">GN-WAAS</span>
              <span className="hidden sm:block text-xs text-gray-500 leading-none">Ghana National Water Audit System</span>
            </div>
          </div>

          {/* Desktop nav */}
          <div className="hidden md:flex items-center gap-8">
            {['Portals', 'Problem', 'Solution', 'How It Works', 'Features', 'Compliance'].map(item => (
              <a
                key={item}
                href={`#${item.toLowerCase().replace(/ /g, '-')}`}
                className="text-sm font-medium text-gray-600 hover:text-green-800 transition-colors"
              >
                {item}
              </a>
            ))}
          </div>

          {/* CTA */}
          <div className="hidden md:flex items-center gap-3">
            <a
              href="https://github.com/ArowuTest/gn-waas"
              target="_blank"
              rel="noopener noreferrer"
              className="text-sm font-medium text-gray-600 hover:text-green-800"
            >
              GitHub
            </a>
            <a
              href="#contact"
              className="bg-green-800 text-white text-sm font-semibold px-4 py-2 rounded-lg hover:bg-green-900 transition-colors"
            >
              Request Demo
            </a>
          </div>

          {/* Mobile menu button */}
          <button className="md:hidden p-2" onClick={() => setOpen(!open)}>
            {open ? <X className="w-5 h-5" /> : <Menu className="w-5 h-5" />}
          </button>
        </div>
      </div>

      {/* Mobile menu */}
      {open && (
        <div className="md:hidden bg-white border-t border-gray-100 px-4 py-4 space-y-3">
          {['Portals', 'Problem', 'Solution', 'How It Works', 'Features', 'Compliance'].map(item => (
            <a
              key={item}
              href={`#${item.toLowerCase().replace(/ /g, '-')}`}
              className="block text-sm font-medium text-gray-700 py-2"
              onClick={() => setOpen(false)}
            >
              {item}
            </a>
          ))}
          <a href="#contact" className="block bg-green-800 text-white text-sm font-semibold px-4 py-2 rounded-lg text-center">
            Request Demo
          </a>
        </div>
      )}
    </nav>
  )
}
