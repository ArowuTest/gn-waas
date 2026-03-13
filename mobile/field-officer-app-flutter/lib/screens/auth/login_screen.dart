// GN-WAAS Field Officer App — Login Screen

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

  @override
  void initState() {
    super.initState();
    _checkBiometric();
  }

  Future<void> _checkBiometric() async {
    final bio = ref.read(biometricServiceProvider);
    final available = await bio.isAvailable();
    if (mounted) setState(() => _biometricAvailable = available);
  }

  /// True when the admin has set require_biometric=true AND the device supports it.
  /// In that case the Sign In button will trigger biometric verification first.
  bool get _biometricRequired {
    final configAsync = ref.read(mobileConfigProvider);
    final requireBio = configAsync.whenOrNull(data: (c) => c.requireBiometric) ?? false;
    return requireBio && _biometricAvailable;
  }

  Future<void> _handleLogin() async {
    if (!_formKey.currentState!.validate()) return;

    // If the admin has mandated biometric and the device supports it,
    // require a biometric pass before accepting the password.
    if (_biometricRequired) {
      final bio = ref.read(biometricServiceProvider);
      final verified = await bio.authenticate(
        reason: 'Biometric verification required to sign in',
      );
      if (!verified) {
        if (mounted) {
          ScaffoldMessenger.of(context).showSnackBar(
            const SnackBar(
              content: Text('Biometric verification failed. Sign-in blocked.'),
              backgroundColor: Colors.red,
            ),
          );
        }
        return;
      }
    }

    await ref.read(authProvider.notifier).login(
      _emailCtrl.text.trim(),
      _passwordCtrl.text,
    );
  }

  Future<void> _handleBiometric() async {
    final api = ref.read(apiServiceProvider);

    // Check if we have a stored refresh token to exchange
    final storedRefresh = await api.getStoredRefreshToken();
    if (storedRefresh == null) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(
            content: Text('Please sign in with your password once to enable biometric login.'),
            backgroundColor: Colors.orange,
          ),
        );
      }
      return;
    }

    final bio = ref.read(biometricServiceProvider);
    final success = await bio.authenticate(
      reason: 'Authenticate to access GN-WAAS Field Officer App',
    );

    if (!success) return;

    // Biometric verified — exchange refresh token for fresh JWT
    if (mounted) {
      setState(() {}); // show loading
    }
    try {
      final data = await api.refreshToken();
      final user = User.fromJson(data['user'] as Map<String, dynamic>);
      final token = (data['access_token'] ?? data['token']) as String;
      await ref.read(authProvider.notifier).loginWithToken(token, user);
    } catch (e) {
      // Refresh token expired — force password login
      await api.logout();
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(
            content: Text('Session expired. Please sign in with your password.'),
            backgroundColor: Colors.red,
          ),
        );
      }
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

    // Show error
    ref.listen<AuthState>(authProvider, (_, next) {
      if (next.error != null) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text(next.error!),
            backgroundColor: Colors.red.shade700,
          ),
        );
      }
    });

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

                // ── Login Button ──────────────────────────────────────────
                ElevatedButton(
                  key: const Key('login_button'),
                  onPressed: auth.isLoading ? null : _handleLogin,
                  style: ElevatedButton.styleFrom(
                    backgroundColor: const Color(0xFF2563eb),
                    foregroundColor: Colors.white,
                    padding: const EdgeInsets.symmetric(vertical: 16),
                    shape: RoundedRectangleBorder(
                      borderRadius: BorderRadius.circular(12),
                    ),
                  ),
                  child: auth.isLoading
                      ? const SizedBox(
                          height: 20, width: 20,
                          child: CircularProgressIndicator(
                            strokeWidth: 2, color: Colors.white,
                          ),
                        )
                      : Text(
                          // When biometric is required by admin policy, signal it
                          _biometricRequired ? 'Sign In (Biometric Required)' : 'Sign In',
                          style: const TextStyle(fontSize: 16, fontWeight: FontWeight.w700),
                        ),
                ),

                // ── Biometric ─────────────────────────────────────────────
                if (_biometricAvailable) ...[
                  const SizedBox(height: 16),
                  OutlinedButton.icon(
                    key: const Key('biometric_button'),
                    onPressed: _handleBiometric,
                    icon: const Icon(Icons.fingerprint, color: Color(0xFF94a3b8)),
                    label: const Text(
                      'Use Biometric',
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
                    style: TextStyle(color: Color(0xFF475569), fontSize: 11, fontWeight: FontWeight.w600),
                  ),
                  const SizedBox(height: 8),
                  Wrap(
                    spacing: 8,
                    runSpacing: 8,
                    alignment: WrapAlignment.center,
                    children: [
                      _devLoginBtn('Field Officer', 'officer.kwame@gnwaas.gov.gh', 'Field@Officer2026!'),
                      _devLoginBtn('GRA Officer', 'graofficer1@gra.gov.gh', 'GRA@Officer2026!'),
                      _devLoginBtn('Field Supervisor', 'supervisor.accra@gnwaas.gov.gh', 'Field@Super2026!'),
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
      _emailCtrl.text = email;
      _passwordCtrl.text = password;
      _handleLogin();
    },
    style: OutlinedButton.styleFrom(
      side: const BorderSide(color: Color(0xFF334155)),
      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(8)),
    ),
    child: Text(label, style: const TextStyle(color: Color(0xFF94a3b8), fontSize: 11)),
  );

  InputDecoration _inputDecoration(String label, IconData icon, {Widget? suffix}) =>
      InputDecoration(
        labelText: label,
        labelStyle: const TextStyle(color: Color(0xFF64748b)),
        prefixIcon: Icon(icon, color: const Color(0xFF64748b)),
        suffixIcon: suffix,
        filled: true,
        fillColor: const Color(0xFF1e293b),
        border: OutlineInputBorder(
          borderRadius: BorderRadius.circular(12),
          borderSide: BorderSide.none,
        ),
        focusedBorder: OutlineInputBorder(
          borderRadius: BorderRadius.circular(12),
          borderSide: const BorderSide(color: Color(0xFF2563eb), width: 2),
        ),
        errorBorder: OutlineInputBorder(
          borderRadius: BorderRadius.circular(12),
          borderSide: const BorderSide(color: Colors.red, width: 1),
        ),
        focusedErrorBorder: OutlineInputBorder(
          borderRadius: BorderRadius.circular(12),
          borderSide: const BorderSide(color: Colors.red, width: 2),
        ),
      );
}
