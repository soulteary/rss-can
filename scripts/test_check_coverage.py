import unittest
from check_coverage import check, summarize


class CoverageGateTests(unittest.TestCase):
    def test_weighted_statements_and_duplicate_blocks(self):
        profile = '\n'.join(['mode: atomic',
            'example/internal/a/a.go:1.1,2.1 9 0',
            'example/internal/a/a.go:1.1,2.1 9 2',
            'example/internal/b/b.go:1.1,2.1 1 0',
            'example/main.go:1.1,2.1 1000 0',
            'example/scripts/tool/main.go:1.1,2.1 1000 0'])
        packages = summarize(profile)
        self.assertEqual(packages, {'a': [9, 9], 'b': [0, 1]})
        self.assertEqual(check(packages, 90, {}), [])
        self.assertTrue(check(packages, 91, {}))

    def test_missing_package_cannot_bypass_gate(self):
        self.assertTrue(check({'a': [100, 100]}, 80, {'missing': 50}))
        self.assertTrue(check({'a': [10, 100]}, 0, {'a': 20}))

    def test_invalid_and_empty_evidence(self):
        for text in ['', 'mode: bad', 'mode: atomic', 'mode: atomic\nbad line',
                     'mode: atomic\nexample/internal/a/a.go:1.1,2.1 -1 1',
                     'mode: atomic\nexample/internal/a/a.go:1.1,2.1 1 -1',
                     'mode: atomic\nexample/internal/a/a.go:1.1,2.1 1 1\nexample/internal/a/a.go:1.1,2.1 2 0']:
            with self.subTest(text=text), self.assertRaises(ValueError):
                summarize(text)

if __name__ == '__main__':
    unittest.main()
