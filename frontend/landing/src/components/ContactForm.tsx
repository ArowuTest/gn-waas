import { useState } from 'react'
import { Send, CheckCircle } from 'lucide-react'

export default function ContactForm() {
  const [submitted, setSubmitted] = useState(false)
  const [form, setForm] = useState({ name: '', org: '', email: '', role: '', message: '' })

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    // In production: POST to /api/v1/contact
    setSubmitted(true)
  }

  return (
    <section id="contact" className="py-24 bg-white">
      <div className="max-w-3xl mx-auto px-4 sm:px-6 lg:px-8">
        <div className="text-center mb-12">
          <div className="inline-block bg-green-100 text-green-800 text-sm font-semibold px-4 py-1.5 rounded-full mb-4">
            Get In Touch
          </div>
          <h2 className="text-3xl sm:text-4xl font-black text-gray-900 mb-4">
            Request a Demo
          </h2>
          <p className="text-gray-600">
            Speak with the GN-WAAS team about a pilot deployment in your district.
          </p>
        </div>

        {submitted ? (
          <div className="bg-green-50 border border-green-200 rounded-2xl p-12 text-center">
            <CheckCircle className="w-16 h-16 text-green-600 mx-auto mb-4" />
            <h3 className="text-xl font-bold text-green-900 mb-2">Message Received</h3>
            <p className="text-green-700">We'll be in touch within 24 hours.</p>
          </div>
        ) : (
          <form onSubmit={handleSubmit} className="bg-gray-50 rounded-2xl p-8 border border-gray-100 space-y-5">
            <div className="grid sm:grid-cols-2 gap-5">
              <div>
                <label className="block text-sm font-semibold text-gray-700 mb-1.5">Full Name *</label>
                <input
                  required
                  type="text"
                  value={form.name}
                  onChange={e => setForm({ ...form, name: e.target.value })}
                  className="w-full border border-gray-200 rounded-lg px-4 py-2.5 text-sm focus:outline-none focus:ring-2 focus:ring-green-500 bg-white"
                  placeholder="Kwame Mensah"
                />
              </div>
              <div>
                <label className="block text-sm font-semibold text-gray-700 mb-1.5">Organisation *</label>
                <input
                  required
                  type="text"
                  value={form.org}
                  onChange={e => setForm({ ...form, org: e.target.value })}
                  className="w-full border border-gray-200 rounded-lg px-4 py-2.5 text-sm focus:outline-none focus:ring-2 focus:ring-green-500 bg-white"
                  placeholder="Ghana Water Limited"
                />
              </div>
            </div>
            <div className="grid sm:grid-cols-2 gap-5">
              <div>
                <label className="block text-sm font-semibold text-gray-700 mb-1.5">Email *</label>
                <input
                  required
                  type="email"
                  value={form.email}
                  onChange={e => setForm({ ...form, email: e.target.value })}
                  className="w-full border border-gray-200 rounded-lg px-4 py-2.5 text-sm focus:outline-none focus:ring-2 focus:ring-green-500 bg-white"
                  placeholder="kwame@gwl.gov.gh"
                />
              </div>
              <div>
                <label className="block text-sm font-semibold text-gray-700 mb-1.5">Your Role</label>
                <select
                  value={form.role}
                  onChange={e => setForm({ ...form, role: e.target.value })}
                  className="w-full border border-gray-200 rounded-lg px-4 py-2.5 text-sm focus:outline-none focus:ring-2 focus:ring-green-500 bg-white"
                >
                  <option value="">Select role...</option>
                  <option>Executive / Director</option>
                  <option>District Manager</option>
                  <option>Finance / Revenue</option>
                  <option>IT / Technical</option>
                  <option>GRA Official</option>
                  <option>PURC Official</option>
                  <option>Other</option>
                </select>
              </div>
            </div>
            <div>
              <label className="block text-sm font-semibold text-gray-700 mb-1.5">Message</label>
              <textarea
                rows={4}
                value={form.message}
                onChange={e => setForm({ ...form, message: e.target.value })}
                className="w-full border border-gray-200 rounded-lg px-4 py-2.5 text-sm focus:outline-none focus:ring-2 focus:ring-green-500 bg-white resize-none"
                placeholder="Tell us about your district and the challenges you're facing..."
              />
            </div>
            <button
              type="submit"
              className="w-full bg-green-800 text-white font-bold py-3 rounded-xl hover:bg-green-900 transition-colors flex items-center justify-center gap-2"
            >
              <Send className="w-4 h-4" />
              Send Message
            </button>
          </form>
        )}
      </div>
    </section>
  )
}
