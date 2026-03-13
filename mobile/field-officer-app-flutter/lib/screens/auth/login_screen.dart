// GN-WAAS Field Officer App — Login Screen
//
// Authentication flow:
//
//   PRIMARY PATH (always available):
//     Email + Password → Sign In
//     This path is NEVER blocked, regardless of admin config.
//
//   OPTIONAL FAST PATH (shown when hardware is available):
//     "Sign in with Fingerprint" button → biometric → refresh-token exchange
//     This is a convenience shortcut, not a gate.
//
//   require_biometric admin config:
//     When true, the biometric button is shown more prominently and the app
//     automatically prompts biometric on screen load (if a refresh token is
//     stored from a previous login).  Password login remains fully accessible.
//     If biometric fails or is cancelled, the officer simply uses the form.
//
//   Rationale (Ghana field deployment):
//     Field officers work outdoors. Dirty/wet/calloused hands frequently fail
//     fingerprint sensors. Budget Android Go devices have unreliable sensors.
//     Blocking login on biometric failure would stop officers doing their jobs.
//     Biometric is therefore always an optional, never a mandatory, step.

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../models/models.dart';
import '../../providers/providers.dart';


class LoginScreen extends ConsumerStatefulWidget {
  const LoginScreen({super.key});

  @override
  ConsumerState<LoginScreen> createState() => _LoginScreenState();
}

class _LoginScreenState extends ConsumerState<LoginScreen> {
  final _formKey        = GlobalKey<FormState>();
  final _emailCtrl      = TextEditingController();
  final _passwordCtrl   = TextEditingController();
  bool  _obscurePassword    = true;
  bool  _biometricAvailable = false;
  bool  _biometricLoading   = false;

  @override
  void initState() {
    super.initState();
    _checkBiometricAndAutoPrompt();
  }

  /// Check hardware availability, then auto-prompt biometric if admin has
  /// enabled require_biometric AND a stored refresh token exists (i.e. the
  /// officer has logged in before and the shortcut is ready to use).
  Future<void> _checkBiometricAndAutoPrompt() async {
    final bio = ref.read(biometricServiceProvider);
    final available = await bio.isAvailable();
    if (!mounted) return;
    setState(() => _biometricAvailable = available);

    if (!available) return;

    // Only auto-prompt if the admin has enabled the convenience setting
    final configAsync = ref.read(mobileConfigProvider);
    final adminWantsBio = configAsync.whenOrNull(data: (c) => c.requireBiometric) ?? false;
    if (!adminWantsBio) return;

    // Only auto-prompt if there is a stored refresh token (i.e. not first login)
    final api = ref.read(apiServiceProvider);
    final storedRefresh = await api.getStoredRefreshToken();
    if (storedRefresh == null || !mounted) return;

    // Auto-prompt — but this is silent/non-blocking.
    // If it fails the form is just shown normally.
    _handleBiometric(autoPrompt: true);
  }

  // ── Password login (primary — always works) ───────────────────────────────

  Future<void> _handleLogin() async {
    if (!_formKey.currentState!.validate()) return;
    await ref.read(authProvider.notifier).login(
      _emailCtrl.text.trim(),
      _passwordCtrl.text,
    );
  }

  // ── Biometric login (optional fast path) ─────────────────────────────────

  Future<void> _handleBiometric({bool autoPrompt = false}) async {
    final api = ref.read(apiServiceProvider);

    // Check we have a refresh token from a previous password login.
    // If not, guide the officer to do a password login first.
    final storedRefresh = await api.getStoredRefreshToken();
    if (storedRefresh == null) {
      if (!autoPrompt && mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(
            content: Text(
              'Sign in with your password once to enable fingerprint login.',
            ),
            backgroundColor: Colors.orange,
            duration: Duration(seconds: 4),
          ),
        );
      }
      return;
    }

    final bio = ref.read(biometricServiceProvider);
    final success = await bio.authenticate(
      reason: 'Use your fingerprint to sign in to GN-WAAS',
    );

    // Biometric cancelled or failed — fall through to password form silently.
    // Never show an error for a failed biometric; just let the form be used.
    if (!success || !mounted) return;

    // Biometric passed — exchange the stored refresh token for a fresh JWT.
    setState(() => _biometricLoading = true);
    try {
      final data = await api.refreshToken();
      if (!mounted) return;
      final user  = User.fromJson(data['user'] as Map<String, dynamic>);
      final token = (data['access_token'] ?? data['token']) as String;
      await ref.read(authProvider.notifier).loginWithToken(token, user);
    } catch (_) {
      // Refresh token has expired — clear it and ask for password.
      await api.logout();
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(
            content: Text('Session expired. Please sign in with your password.'),
            backgroundColor: Colors.orange,
            duration: Duration(seconds: 4),
          ),
        );
      }
    } finally {
      if (mounted) setState(() => _biometricLoading = false);
    }
  }

  @override
  void dispose() {
    _emailCtrl.dispose();
    _passwordCtrl.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final auth = ref.watch(authProvider);

    // Show server-side login errors
    ref.listen<AuthState>(authProvider, (_, next) {
      if (next.error != null && mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text(next.error!),
            backgroundColor: Colors.red.shade700,
          ),
        );
      }
    });

    // Is biometric promoted by admin policy?
    final adminPromotesBio =
        (ref.watch(mobileConfigProvider).whenOrNull(data: (c) => c.requireBiometric) ?? false) &&
        _biometricAvailable;

    final isLoading = auth.isLoading || _biometricLoading;

    return Scaffold(
      backgroundColor: const Color(0xFF0f172a),
      body: SafeArea(
        child: SingleChildScrollView(
          padding: const EdgeInsets.all(24),
          child: Form(
            key: _formKey,
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                const SizedBox(height: 48),

                // ── Logo ──────────────────────────────────────────────────
                Center(
                  child: Container(
                    width: 80, height: 80,
                    decoration: BoxDecoration(
                      color: const Color(0xFF1e3a5f),
                      borderRadius: BorderRadius.circular(20),
                    ),
                    child: const Center(
                      child: Text('💧', style: TextStyle(fontSize: 40)),
                    ),
                  ),
                ),
                const SizedBox(height: 16),
                const Center(
                  child: Text(
                    'GN-WAAS',
                    style: TextStyle(
                      color: Colors.white,
                      fontSize: 28,
                      fontWeight: FontWeight.w900,
                      letterSpacing: 2,
                    ),
                  ),
                ),
                const Center(
                  child: Text(
                    'Field Officer App',
                    style: TextStyle(color: Color(0xFF94a3b8), fontSize: 14),
                  ),
                ),
                const SizedBox(height: 48),

                // ── Biometric shortcut (promoted position when admin-enabled)
                // Shown above the form so officers who have already logged in
                // once can tap it immediately rather than re-typing credentials.
                if (_biometricAvailable && adminPromotesBio) ...[
                  OutlinedButton.icon(
                    key: const Key('biometric_button_top'),
                    onPressed: isLoading ? null : _handleBiometric,
                    icon: _biometricLoading
                        ? const SizedBox(
                            width: 18, height: 18,
                            child: CircularProgressIndicator(
                              strokeWidth: 2, color: Color(0xFF94a3b8),
                            ),
                          )
                        : const Icon(Icons.fingerprint, color: Color(0xFF60a5fa), size: 26),
                    label: const Text(
                      'Sign in with Fingerprint',
                      style: TextStyle(
                        color: Color(0xFF60a5fa),
                        fontSize: 15,
                        fontWeight: FontWeight.w600,
                      ),
                    ),
                    style: OutlinedButton.styleFrom(
                      side: const BorderSide(color: Color(0xFF1d4ed8), width: 1.5),
                      padding: const EdgeInsets.symmetric(vertical: 16),
                      shape: RoundedRectangleBorder(
                        borderRadius: BorderRadius.circular(12),
                      ),
                    ),
                  ),
                  const SizedBox(height: 20),
                  Row(
                    children: const [
                      Expanded(child: Divider(color: Color(0xFF1e293b))),
                      Padding(
                        padding: EdgeInsets.symmetric(horizontal: 12),
                        child: Text(
                          'or sign in with password',
                          style: TextStyle(color: Color(0xFF475569), fontSize: 12),
                        ),
                      ),
                      Expanded(child: Divider(color: Color(0xFF1e293b))),
                    ],
                  ),
                  const SizedBox(height: 20),
                ],

                // ── Email ─────────────────────────────────────────────────
                TextFormField(
                  key: const Key('email_field'),
                  controller: _emailCtrl,
                  keyboardType: TextInputType.emailAddress,
                  autocorrect: false,
                  style: const TextStyle(color: Colors.white),
                  decoration: _inputDecoration('Email Address', Icons.email_outlined),
                  validator: (v) {
                    if (v == null || v.isEmpty) return 'Email is required';
                    if (!v.contains('@')) return 'Enter a valid email';
                    return null;
                  },
                ),
                const SizedBox(height: 16),

                // ── Password ──────────────────────────────────────────────
                TextFormField(
                  key: const Key('password_field'),
                  controller: _passwordCtrl,
                  obscureText: _obscurePassword,
                  style: const TextStyle(color: Colors.white),
                  decoration: _inputDecoration(
                    'Password',
                    Icons.lock_outline,
                    suffix: IconButton(
                      icon: Icon(
                        _obscurePassword ? Icons.visibility_off : Icons.visibility,
                        color: const Color(0xFF64748b),
                      ),
                      onPressed: () =>
                          setState(() => _obscurePassword = !_obscurePassword),
                    ),
                  ),
                  validator: (v) {
                    if (v == null || v.isEmpty) return 'Password is required';
                    if (v.length < 6) return 'Password must be at least 6 characters';
                    return null;
                  },
                ),
                const SizedBox(height: 32),

                // ── Sign In Button (always available) ─────────────────────
                ElevatedButton(
                  key: const Key('login_button'),
                  onPressed: isLoading ? null : _handleLogin,
                  style: ElevatedButton.styleFrom(
                    backgroundColor: const Color(0xFF2563eb),
                    foregroundColor: Colors.white,
                    padding: const EdgeInsets.symmetric(vertical: 16),
                    shape: RoundedRectangleBorder(
                      borderRadius: BorderRadius.circular(12),
                    ),
                  ),
                  child: isLoading
                      ? const SizedBox(
                          height: 20, width: 20,
                          child: CircularProgressIndicator(
                            strokeWidth: 2, color: Colors.white,
                          ),
                        )
                      : const Text(
                          'Sign In',
                          style: TextStyle(
                            fontSize: 16, fontWeight: FontWeight.w700,
                          ),
                        ),
                ),

                // ── Biometric shortcut (secondary position, non-promoted)
                // Shown below Sign In when hardware is available but admin has
                // not promoted it.  Officer can optionally use it after the
                // first password login has stored a refresh token.
                if (_biometricAvailable && !adminPromotesBio) ...[
                  const SizedBox(height: 16),
                  OutlinedButton.icon(
                    key: const Key('biometric_button'),
                    onPressed: isLoading ? null : _handleBiometric,
                    icon: const Icon(Icons.fingerprint, color: Color(0xFF94a3b8)),
                    label: const Text(
                      'Use Fingerprint',
                      style: TextStyle(color: Color(0xFF94a3b8)),
                    ),
                    style: OutlinedButton.styleFrom(
                      side: const BorderSide(color: Color(0xFF334155)),
                      padding: const EdgeInsets.symmetric(vertical: 14),
                      shape: RoundedRectangleBorder(
                        borderRadius: BorderRadius.circular(12),
                      ),
                    ),
                  ),
                ],

                const SizedBox(height: 32),

                // ── Dev Quick Login (staging only) ────────────────────────
                // SECURITY NOTE: These buttons pre-fill test credentials.
                // They are HIDDEN by default (defaultValue: false).
                // To enable for a staging build use:
                //   flutter run --dart-define=SHOW_DEV_LOGIN=true
                // They must NEVER appear in a production APK/IPA.
                if (const bool.fromEnvironment('SHOW_DEV_LOGIN', defaultValue: false)) ...[
                  const Divider(color: Color(0xFF1e293b), height: 32),
                  const Text(
                    'Dev Quick Login',
                    style: TextStyle(
                      color: Color(0xFF475569),
                      fontSize: 11,
                      fontWeight: FontWeight.w600,
                    ),
                  ),
                  const SizedBox(height: 8),
                  Wrap(
                    spacing: 8,
                    runSpacing: 8,
                    alignment: WrapAlignment.center,
                    children: [
                      _devLoginBtn('Field Officer',  'officer.kwame@gnwaas.gov.gh',    'Field@Officer2026!'),
                      _devLoginBtn('GRA Officer',    'graofficer1@gra.gov.gh',          'GRA@Officer2026!'),
                      _devLoginBtn('Supervisor',     'supervisor.accra@gnwaas.gov.gh', 'Field@Super2026!'),
                    ],
                  ),
                  const SizedBox(height: 16),
                ],

                const Center(
                  child: Text(
                    'Ghana National Water Audit & Assurance System\n© 2026 Ghana Water Limited',
                    textAlign: TextAlign.center,
                    style: TextStyle(color: Color(0xFF475569), fontSize: 11),
                  ),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }

  Widget _devLoginBtn(String label, String email, String password) => OutlinedButton(
    onPressed: () {
      _emailCtrl.text    = email;
      _passwordCtrl.text = password;
      _handleLogin();
    },
    style: OutlinedButton.styleFrom(
      side: const BorderSide(color: Color(0xFF334155)),
      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(8)),
    ),
    child: Text(
      label,
      style: const TextStyle(color: Color(0xFF94a3b8), fontSize: 11),
    ),
  );

  InputDecoration _inputDecoration(
    String label,
    IconData icon, {
    Widget? suffix,
  }) =>
      InputDecoration(
        labelText:  label,
        labelStyle: const TextStyle(color: Color(0xFF64748b)),
        prefixIcon: Icon(icon, color: const Color(0xFF64748b)),
        suffixIcon: suffix,
        filled:     true,
        fillColor:  const Color(0xFF1e293b),
        border: OutlineInputBorder(
          borderRadius: BorderRadius.circular(12),
          borderSide:   BorderSide.none,
        ),
        focusedBorder: OutlineInputBorder(
          borderRadius: BorderRadius.circular(12),
          borderSide:   const BorderSide(color: Color(0xFF2563eb), width: 2),
        ),
        errorBorder: OutlineInputBorder(
          borderRadius: BorderRadius.circular(12),
          borderSide:   const BorderSide(color: Colors.red, width: 1),
        ),
        focusedErrorBorder: OutlineInputBorder(
          borderRadius: BorderRadius.circular(12),
          borderSide:   const BorderSide(color: Colors.red, width: 2),
        ),
      );
}
